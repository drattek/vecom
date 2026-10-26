// Package meli_bulk_import imports MercadoLibre listings in bulk from an Excel
// file with two columns (sku, meli_id). Each row becomes a queued item
// (ecom_meli_import_items) that a background worker imports one at a time, with
// the exact flow of the single "Importar publicación existente" button
// (meli_listing_import): validate the listing's part number (or, without one,
// its seller SKU), then import name, category, description, attributes,
// pictures and compatibilities and link the listing.
//
// The bulk-only rules: a SKU that doesn't exist in ecom_products is recorded as
// "missing" and never imported, and a product that already has a listing linked
// on the connection is recorded as "duplicate" and skipped.
package meli_bulk_import

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"

	meliListingImportApp "core-orchestrator/internal/application/meli_listing_import"
	syncApp "core-orchestrator/internal/application/sync"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"

	"github.com/xuri/excelize/v2"
)

// maxRows bounds one file so a mistaken upload can't queue an unbounded import.
const maxRows = 5000

var (
	ErrInvalidInput       = errors.New("connectionId and an Excel file are required")
	ErrInvalidFile        = errors.New("the file is not a valid Excel (.xlsx) file")
	ErrMissingColumns     = errors.New("the Excel must have the columns sku and meli_id in its first row")
	ErrEmptyFile          = errors.New("the Excel has no rows to import")
	ErrTooManyRows        = fmt.Errorf("the Excel has more than %d rows", maxRows)
	ErrConnectionNotFound = mysqlInfra.ErrChannelConnectionNotFound
	ErrItemNotRetryable   = errors.New("only failed or missing items can be retried")
	ErrStillMissing       = errors.New("the SKU still doesn't exist in the products table")
)

// listingImporter is the single-listing import this service drives.
type listingImporter interface {
	RunSync(ctx context.Context, input meliListingImportApp.ImportInput) (*meliListingImportApp.Result, error)
}

type Service struct {
	db                          *sql.DB
	repo                        *mysqlInfra.MeliImportRepository
	productRepository           *mysqlInfra.ProductRepository
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository
	channelRepository           *mysqlInfra.ChannelRepository
	importer                    listingImporter
	tokenService                *syncApp.MercadoLibreTokenService
	itemsHandler                *mercadoLibreInfra.ItemsHandler
}

// ListBatches returns the most recent uploaded files, newest first.
func (s *Service) ListBatches(ctx context.Context, connectionID int64) ([]mysqlInfra.MeliImportBatchDTO, error) {
	return s.repo.ListBatches(ctx, connectionID, 200)
}

func NewService(
	db *sql.DB,
	repo *mysqlInfra.MeliImportRepository,
	productRepository *mysqlInfra.ProductRepository,
	channelConnectionRepository *mysqlInfra.ChannelConnectionRepository,
	channelRepository *mysqlInfra.ChannelRepository,
	importer *meliListingImportApp.Service,
	tokenService *syncApp.MercadoLibreTokenService,
	rateLimiter *mercadoLibreInfra.RateLimiter,
) *Service {
	return &Service{
		tokenService:                tokenService,
		itemsHandler:                mercadoLibreInfra.NewItemsHandler(mercadoLibreInfra.NewClient(nil, "", rateLimiter)),
		db:                          db,
		repo:                        repo,
		productRepository:           productRepository,
		channelConnectionRepository: channelConnectionRepository,
		channelRepository:           channelRepository,
		importer:                    importer,
	}
}

type UploadInput struct {
	ConnectionID int64
	FileName     string
	File         io.Reader
	ActorID      int64
}

// UploadResult counts how the rows of a new batch were classified.
type UploadResult struct {
	BatchID   int64 `json:"batchId"`
	TotalRows int   `json:"totalRows"`
	Queued    int   `json:"queued"`
	Missing   int   `json:"missing"`
	Invalid   int   `json:"invalid"`
}

type sheetRow struct {
	number int
	sku    string
	meliID string
}

// Upload parses the Excel and records the batch: rows whose SKU exists are
// queued for the worker, rows whose SKU doesn't are recorded as "missing", and
// rows with a blank sku or an unusable meli_id as "failed". Nothing is imported
// here — the worker does it afterwards.
func (s *Service) Upload(ctx context.Context, in UploadInput) (*UploadResult, error) {
	if in.ConnectionID <= 0 || in.ActorID <= 0 || in.File == nil {
		return nil, ErrInvalidInput
	}
	connection, err := s.channelConnectionRepository.FindByID(ctx, in.ConnectionID)
	if err != nil {
		return nil, err
	}
	channel, err := s.channelRepository.FindByID(ctx, connection.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("error loading channel %d: %w", connection.ChannelID, err)
	}
	if !strings.EqualFold(strings.TrimSpace(channel.Code), "MERCADOLIBRE") {
		return nil, syncApp.ErrNotMercadoLibreConnection
	}

	rows, err := parseSheet(in.File)
	if err != nil {
		return nil, err
	}

	type resolved struct {
		sheetRow
		externalID string
		productID  *int64
		status     string
		message    string
	}
	items := make([]resolved, 0, len(rows))
	result := &UploadResult{TotalRows: len(rows)}
	for _, row := range rows {
		item := resolved{sheetRow: row, status: mysqlInfra.MeliImportPending}
		switch {
		case row.sku == "" || row.meliID == "":
			item.status, item.message = mysqlInfra.MeliImportFailed, "La fila debe tener sku y meli_id."
			result.Invalid++
		default:
			externalID, err := meliListingImportApp.NormalizeItemID(row.meliID)
			if err != nil {
				item.status, item.message = mysqlInfra.MeliImportFailed, "El meli_id no es válido; usa el formato MLM123456789."
				result.Invalid++
				break
			}
			item.externalID = externalID

			product, err := s.productRepository.FindBySKU(ctx, row.sku)
			switch {
			case err == nil:
				item.productID = &product.ID
				result.Queued++
			case errors.Is(err, mysqlInfra.ErrProductNotFound):
				item.status, item.message = mysqlInfra.MeliImportMissing, "El SKU no existe en la tabla de productos."
				result.Missing++
			default:
				return nil, fmt.Errorf("error looking up sku %q: %w", row.sku, err)
			}
		}
		items = append(items, item)
	}

	err = mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		repo := mysqlInfra.NewMeliImportRepository(tx)
		batchID, err := repo.CreateBatch(ctx, mysqlInfra.CreateMeliImportBatchInput{
			ConnectionID: in.ConnectionID,
			FileName:     in.FileName,
			TotalRows:    len(items),
			ActorID:      in.ActorID,
		})
		if err != nil {
			return err
		}
		result.BatchID = batchID
		for _, item := range items {
			if err := repo.CreateItem(ctx, mysqlInfra.CreateMeliImportItemInput{
				BatchID:      batchID,
				ConnectionID: in.ConnectionID,
				SheetRow:     item.number,
				SKU:          item.sku,
				MeliIDRaw:    item.meliID,
				ExternalID:   item.externalID,
				ProductID:    item.productID,
				Status:       item.status,
				ErrorMessage: item.message,
				ActorID:      in.ActorID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// parseSheet reads the first sheet: the header row must contain the columns
// sku and meli_id (any order, any case); fully empty rows are skipped.
func parseSheet(file io.Reader) ([]sheetRow, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading the uploaded file: %w", err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalidFile
	}
	defer book.Close()

	sheets := book.GetSheetList()
	if len(sheets) == 0 {
		return nil, ErrInvalidFile
	}
	all, err := book.GetRows(sheets[0])
	if err != nil {
		return nil, ErrInvalidFile
	}

	skuCol, idCol := -1, -1
	headerRow := -1
	for i, row := range all {
		for c, cell := range row {
			switch strings.ToLower(strings.TrimSpace(cell)) {
			case "sku":
				skuCol = c
			case "meli_id":
				idCol = c
			}
		}
		if skuCol >= 0 && idCol >= 0 {
			headerRow = i
			break
		}
		skuCol, idCol = -1, -1
	}
	if headerRow < 0 {
		return nil, ErrMissingColumns
	}

	cell := func(row []string, col int) string {
		if col < len(row) {
			return strings.TrimSpace(row[col])
		}
		return ""
	}
	var rows []sheetRow
	for i := headerRow + 1; i < len(all); i++ {
		sku, meliID := cell(all[i], skuCol), cell(all[i], idCol)
		if sku == "" && meliID == "" {
			continue
		}
		rows = append(rows, sheetRow{number: i + 1, sku: sku, meliID: meliID})
		if len(rows) > maxRows {
			return nil, ErrTooManyRows
		}
	}
	if len(rows) == 0 {
		return nil, ErrEmptyFile
	}
	return rows, nil
}

func (s *Service) List(ctx context.Context, f mysqlInfra.MeliImportItemFilter) (*mysqlInfra.MeliImportItemsPage, error) {
	return s.repo.FindItems(ctx, f)
}

func (s *Service) Summary(ctx context.Context, connectionID, batchID int64) (map[string]int, error) {
	return s.repo.CountByStatus(ctx, connectionID, batchID)
}

// Retry puts a failed or missing item back in the queue. A missing item first
// looks its SKU up again: it is retryable only once the product exists.
func (s *Service) Retry(ctx context.Context, itemID, actorID int64) error {
	item, err := s.repo.FindItem(ctx, itemID)
	if err != nil {
		return err
	}
	if item.Status != mysqlInfra.MeliImportFailed && item.Status != mysqlInfra.MeliImportMissing {
		return ErrItemNotRetryable
	}
	return s.requeue(ctx, item, actorID)
}

// RetryAll requeues every item in status (failed or missing). Items whose SKU
// still doesn't exist stay missing; the number actually requeued is returned.
func (s *Service) RetryAll(ctx context.Context, status string, connectionID, actorID int64) (int, error) {
	if status != mysqlInfra.MeliImportFailed && status != mysqlInfra.MeliImportMissing {
		return 0, ErrItemNotRetryable
	}
	items, err := s.repo.ListByStatus(ctx, status, connectionID)
	if err != nil {
		return 0, err
	}
	requeued := 0
	for i := range items {
		if err := s.requeue(ctx, &items[i], actorID); err == nil {
			requeued++
		} else if !errors.Is(err, ErrStillMissing) {
			return requeued, err
		}
	}
	return requeued, nil
}

func (s *Service) requeue(ctx context.Context, item *mysqlInfra.MeliImportItemDTO, actorID int64) error {
	productID := item.ProductID
	if productID == nil || item.Status == mysqlInfra.MeliImportMissing {
		product, err := s.productRepository.FindBySKU(ctx, item.SKU)
		if errors.Is(err, mysqlInfra.ErrProductNotFound) {
			return ErrStillMissing
		}
		if err != nil {
			return err
		}
		productID = &product.ID
	}
	return s.repo.Requeue(ctx, item.ID, productID, actorID)
}
