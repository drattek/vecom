package meli_bulk_import

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"time"

	meliListingImportApp "core-orchestrator/internal/application/meli_listing_import"
	syncApp "core-orchestrator/internal/application/sync"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/shared/safe"
)

const (
	// pollInterval es lo que espera el worker cuando la cola está vacía.
	pollInterval = 5 * time.Second
	// staleRunningAfter es lo que puede estar un ítem en "running" antes de
	// asumir que pertenece a un proceso muerto (un import dura a lo sumo 35 min).
	staleRunningAfter = 45 * time.Minute
)

// Start es el loop del worker: importa los ítems en cola estrictamente de uno
// en uno (el presupuesto de la API de MercadoLibre es compartido y el tiempo
// total de un import masivo no importa) hasta que ctx se cancela. Se corre bajo
// safe.Supervise.
func (s *Service) Start(ctx context.Context) {
	log.Printf("meli bulk import worker: started")
	for ctx.Err() == nil {
		if n, err := s.repo.RequeueStale(ctx, staleRunningAfter); err != nil {
			log.Printf("meli bulk import worker: %v", err)
		} else if n > 0 {
			log.Printf("meli bulk import worker: requeued %d stale item(s)", n)
		}

		claim, err := s.claimNext(ctx)
		if err != nil {
			log.Printf("meli bulk import worker: %v", err)
		}
		if claim == nil {
			select {
			case <-ctx.Done():
			case <-time.After(pollInterval):
			}
			continue
		}

		if !safe.Do("meli-bulk-import-item", func() { s.process(ctx, claim) }) {
			s.finish(claim, mysqlInfra.MeliImportOutcome{
				Status:       mysqlInfra.MeliImportFailed,
				ErrorMessage: "Error interno durante la importación (ver el log del servidor).",
				ActorID:      claim.ActorID,
			})
		}
	}
	log.Printf("meli bulk import worker: stopped")
}

func (s *Service) claimNext(ctx context.Context) (*mysqlInfra.MeliImportClaim, error) {
	var claim *mysqlInfra.MeliImportClaim
	err := mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		claim, err = mysqlInfra.NewMeliImportRepository(tx).ClaimNext(ctx)
		return err
	})
	return claim, err
}

// process importa un ítem tomado de la cola y registra su resultado.
func (s *Service) process(ctx context.Context, claim *mysqlInfra.MeliImportClaim) {
	outcome := mysqlInfra.MeliImportOutcome{ActorID: claim.ActorID}

	productID := int64(0)
	if claim.ProductID != nil {
		productID = *claim.ProductID
	} else if product, err := s.productRepository.FindBySKU(ctx, claim.SKU); err == nil {
		productID = product.ID
	}
	if productID == 0 {
		outcome.Status, outcome.ErrorMessage = mysqlInfra.MeliImportMissing, "El SKU no existe en la tabla de productos."
		s.finish(claim, outcome)
		return
	}

	result, err := s.importer.RunSync(ctx, meliListingImportApp.ImportInput{
		ProductID:    productID,
		ConnectionID: claim.ConnectionID,
		ExternalID:   claim.ExternalID,
		ActorID:      claim.ActorID,
	})
	if result != nil {
		outcome.ListingTitle = result.Title
		outcome.Steps, _ = json.Marshal(result.Steps)
	}

	switch {
	case err == nil && result.Linked:
		outcome.Status = mysqlInfra.MeliImportDone
	case err == nil:
		outcome.Status = mysqlInfra.MeliImportFailed
		outcome.ErrorMessage = "No se vinculó la publicación porque hubo errores al guardar la información; revisa los pasos y reintenta."
	case errors.Is(err, meliListingImportApp.ErrProductAlreadyListed):
		outcome.Status = mysqlInfra.MeliImportDuplicate
		outcome.ErrorMessage = "El producto ya tiene una publicación vinculada en esta conexión."
	case errors.Is(err, meliListingImportApp.ErrProductNotFound):
		outcome.Status = mysqlInfra.MeliImportMissing
		outcome.ErrorMessage = "El SKU no existe en la tabla de productos."
	default:
		outcome.Status = mysqlInfra.MeliImportFailed
		outcome.ErrorMessage = describeError(err)
	}
	s.finish(claim, outcome)
}

// finish guarda el resultado con un contexto nuevo: el del worker puede estar
// cancelado por un apagado, y el resultado de un import que ya corrió no debe
// perderse.
func (s *Service) finish(claim *mysqlInfra.MeliImportClaim, outcome mysqlInfra.MeliImportOutcome) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.repo.SetOutcome(ctx, claim.ID, outcome); err != nil {
		log.Printf("meli bulk import worker: %v", err)
		return
	}
	log.Printf("meli bulk import worker: item=%d sku=%s status=%s", claim.ID, claim.SKU, outcome.Status)
}

func describeError(err error) string {
	var mismatch *meliListingImportApp.PartNumberMismatchError
	switch {
	case errors.As(err, &mismatch):
		return mismatch.Message()
	case errors.Is(err, meliListingImportApp.ErrListingLinkedToOtherProduct):
		return "La publicación ya está vinculada a otro producto."
	case errors.Is(err, meliListingImportApp.ErrListingNotFound):
		return "La publicación no existe en MercadoLibre."
	case errors.Is(err, meliListingImportApp.ErrListingNotOwned):
		return "La publicación no pertenece a la cuenta de MercadoLibre de esta conexión."
	case errors.Is(err, meliListingImportApp.ErrListingClosed):
		return "La publicación está cerrada en MercadoLibre y no se puede importar."
	case errors.Is(err, meliListingImportApp.ErrInvalidExternalID):
		return "El meli_id no es válido; usa el formato MLM123456789."
	case errors.Is(err, syncApp.ErrNotMercadoLibreConnection):
		return "La conexión no es de MercadoLibre."
	case errors.Is(err, meliListingImportApp.ErrListingFetch):
		return "No se pudo consultar la publicación en MercadoLibre. Reintenta más tarde."
	default:
		log.Printf("meli bulk import worker: %v", err)
		return "Error al importar la publicación: " + err.Error()
	}
}
