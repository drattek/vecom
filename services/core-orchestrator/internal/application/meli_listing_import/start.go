package meli_listing_import

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	productCategorySelectionApp "core-orchestrator/internal/application/product_category_selection"
	syncApp "core-orchestrator/internal/application/sync"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/shared/safe"
)

// prepared is an import that passed validation and is ready to run.
type prepared struct {
	input       ImportInput
	externalID  string
	product     *mysqlInfra.ProductDTO
	accessToken string
	item        *mercadoLibreInfra.ItemDetail
	mapStatus   string
}

// Start validates the import and, if it is valid, runs it in the background,
// returning the job to poll (GetJob). Errors returned here mean nothing was
// written (validation failed). From then on the import is independent of the
// HTTP request that started it: it can take longer than any proxy in front of
// the API waits for a response (nginx's 60 s, Cloudflare's 100 s -> error 524),
// so progress is reported through the job instead of the response. Each local
// write is idempotent and the listing is linked only after all of them
// succeeded, so after fixing a reported problem the import can simply be run
// again.
func (s *Service) Start(ctx context.Context, input ImportInput) (*Job, error) {
	p, err := s.prepare(ctx, input)
	if err != nil {
		return nil, err
	}

	jobID, err := newJobID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	job := &Job{
		ID:           jobID,
		ProductID:    p.product.ID,
		ConnectionID: input.ConnectionID,
		Status:       JobRunning,
		Result: &Result{
			ProductID:    p.product.ID,
			ConnectionID: input.ConnectionID,
			ExternalID:   p.externalID,
			Title:        strings.TrimSpace(p.item.Title),
			CategoryID:   p.item.CategoryID,
			Steps:        []StepResult{},
		},
		StartedAt: now,
		UpdatedAt: now,
	}

	payload, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("error encoding import job: %w", err)
	}
	if err := s.jobs.Save(ctx, job.ID, payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrJobStore, err)
	}

	// The goroutine owns job from here on; the caller gets a copy.
	response := job.snapshot()
	go func() {
		// Not derived from the request context: it is cancelled as soon as the
		// client (or a proxy giving up on it) disconnects.
		runCtx, cancel := context.WithTimeout(context.Background(), importTimeout)
		defer cancel()

		if !safe.Do("meli-listing-import", func() { s.execute(runCtx, p, job) }) {
			job.Status = JobFailed
			job.Error = "Error interno durante la importación (ver el log del servidor)."
			s.saveJob(job)
		}
	}()

	return response, nil
}

// prepare runs every check that can reject an import before anything is
// written: the product, the connection, existing listings, then the listing
// itself on MercadoLibre (it exists, is alive, belongs to the connection's
// account and carries the product's part number).
func (s *Service) prepare(ctx context.Context, input ImportInput) (*prepared, error) {
	if input.ProductID <= 0 || input.ConnectionID <= 0 || input.ActorID <= 0 || strings.TrimSpace(input.ExternalID) == "" {
		return nil, ErrInvalidInput
	}
	externalID, err := NormalizeItemID(input.ExternalID)
	if err != nil {
		return nil, err
	}

	product, err := s.productRepository.FindByID(ctx, input.ProductID)
	if err != nil {
		return nil, err
	}

	if err := s.requireMercadoLibreConnection(ctx, input.ConnectionID); err != nil {
		return nil, err
	}

	active, err := s.channelProductMapRepository.CountActiveListings(ctx, product.ID, input.ConnectionID)
	if err != nil {
		return nil, err
	}
	if active > 0 {
		return nil, ErrProductAlreadyListed
	}

	if existing, err := s.channelProductMapRepository.FindByConnectionAndExternalID(ctx, input.ConnectionID, externalID); err == nil {
		if existing.ProductID != product.ID {
			return nil, fmt.Errorf("%w (product %d)", ErrListingLinkedToOtherProduct, existing.ProductID)
		}
	} else if !errors.Is(err, mysqlInfra.ErrChannelProductMapNotFound) {
		return nil, fmt.Errorf("error looking up listing %s: %w", externalID, err)
	}

	accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, input.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("error getting mercadolibre access token: %w", err)
	}

	item, err := s.itemsHandler.GetItem(ctx, accessToken, externalID)
	if err != nil {
		// parseItemAPIError doesn't expose a typed status, only "(404): ...".
		if strings.Contains(err.Error(), "(404)") {
			return nil, fmt.Errorf("%w: %s", ErrListingNotFound, externalID)
		}
		return nil, fmt.Errorf("%w (%s): %v", ErrListingFetch, externalID, err)
	}
	// A listing MercadoLibre reports as permanently dead (closed, or
	// under_review/forbidden) can't be recovered, so there is nothing to link.
	mapStatus := syncApp.FoldMercadoLibreItemStatus(item.Status, item.SubStatus)
	if strings.EqualFold(item.Status, meliStatusClosed) || mapStatus == meliMapStatusClosed {
		return nil, ErrListingClosed
	}

	me, err := s.usersHandler.GetMe(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("error resolving the connection's mercadolibre account: %w", err)
	}
	if item.SellerID != me.ID {
		return nil, ErrListingNotOwned
	}

	if err := validatePartNumber(product.PartNumber, item.Attributes); err != nil {
		return nil, err
	}

	return &prepared{input: input, externalID: externalID, product: product, accessToken: accessToken, item: item, mapStatus: mapStatus}, nil
}

// execute runs the import steps for a validated import, saving the job after
// each one so the dashboard sees the progress, and marks it done at the end.
func (s *Service) execute(ctx context.Context, p *prepared, job *Job) {
	input, externalID, product, item, accessToken, mapStatus := p.input, p.externalID, p.product, p.item, p.accessToken, p.mapStatus
	result := job.Result

	failed := false
	started := time.Now()
	logf := func(format string, args ...any) {
		log.Printf("meli listing import: product=%d item=%s: "+format, append([]any{product.ID, externalID}, args...)...)
	}
	record := func(step, status, detail string) {
		result.Steps = append(result.Steps, StepResult{Step: step, Status: status, Detail: detail})
		if status == StatusError {
			failed = true
		}
		logf("step=%s status=%s elapsed=%s detail=%s", step, status, time.Since(started).Round(time.Millisecond), detail)
		s.saveJob(job)
	}
	// run logs when a step starts, so if the import stalls the last "started"
	// line without a matching result is the step it is stuck in.
	run := func(step string, fn func()) {
		logf("step=%s started", step)
		fn()
	}
	// Marks the job done once execute returns, whichever way it does.
	defer func() {
		job.Status = JobDone
		s.saveJob(job)
		logf("finished in %s (linked=%t)", time.Since(started).Round(time.Millisecond), result.Linked)
	}()
	logf("started (part number validated)")

	run(StepName, func() { s.importName(ctx, product, item, input.ActorID, record) })
	var selection *productCategorySelectionApp.SelectionDTO
	run(StepCategory, func() { selection = s.importCategory(ctx, product, item, input, record) })
	run(StepDescription, func() { s.importDescription(ctx, product, item, accessToken, input.ActorID, record) })
	run(StepAttributes, func() { s.importAttributes(ctx, product, item, selection, input, record) })
	run(StepCompatibilities, func() { s.importCompatibilities(ctx, product, externalID, accessToken, input.ActorID, record) })

	if failed {
		record(StepLink, StatusSkipped, "No se vinculó la publicación porque hubo errores al guardar la información; corrígelos y vuelve a intentar.")
		return
	}

	pausedReason := ""
	if mapStatus == meliMapStatusPaused {
		// Paused by someone other than this system: never auto-reactivated.
		pausedReason = pausedReasonUser
	}
	listing, err := s.channelProductMapRepository.Upsert(ctx, mysqlInfra.UpsertChannelProductMapInput{
		ProductID:          product.ID,
		ConnectionID:       input.ConnectionID,
		ListingTitle:       result.Title,
		ExternalID:         externalID,
		ExternalCategoryID: item.CategoryID,
		Status:             mapStatus,
		PausedReason:       pausedReason,
		ActorID:            input.ActorID,
	})
	if err != nil {
		record(StepLink, StatusError, fmt.Sprintf("No se pudo registrar la publicación: %v", err))
		return
	}
	result.Linked = true
	result.Status = mapStatus
	record(StepLink, StatusOK, fmt.Sprintf("Publicación %s vinculada (estado: %s).", externalID, mapStatus))

	refreshed, err := s.productRepository.FindByID(ctx, product.ID)
	if err != nil {
		refreshed = product
	}
	run(StepPriceStock, func() {
		push := s.productSyncService.PushImportedListing(ctx, refreshed, input.ConnectionID, listing, item)
		recordPush(push, record)
	})

	run(StepListingType, func() {
		recordNormalize(s.productSyncService.NormalizeImportedListing(ctx, input.ConnectionID, listing, item), record)
	})

	// Pictures go last: they are by far the slowest step (download, lossless
	// WebP conversion, upload) and supplementary, so the listing is already
	// linked and up to date in MercadoLibre by the time they run.
	run(StepImages, func() { s.importImages(ctx, product, item, input.ActorID, record) })
}
