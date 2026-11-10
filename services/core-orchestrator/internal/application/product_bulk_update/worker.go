package product_bulk_update

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	channelListingsApp "core-orchestrator/internal/application/channel_listings"
	productImageImportApp "core-orchestrator/internal/application/product_image_import"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	"core-orchestrator/internal/shared/safe"
)

const (
	// workers es cuántas filas se procesan a la vez. Cada fila hace varias
	// peticiones HEAD a las URLs de fotos y video, así que paralelizar ahorra
	// mucho tiempo; ClaimNext usa SKIP LOCKED y los workers no se pisan.
	workers = 4
	// pollInterval es lo que espera un worker cuando la cola está vacía.
	pollInterval = 5 * time.Second
	// staleRunningAfter es lo que puede estar una fila en "running" antes de
	// asumir que pertenece a un proceso muerto (una fila tarda segundos).
	staleRunningAfter = 15 * time.Minute
)

// Pasos de una fila, en el orden en que se ejecutan. Se guardan en el ítem
// (columna steps) para que la vista y el reporte muestren qué pasó en cada uno.
const (
	StepGeneral    = "general"
	StepSEO        = "seo"
	StepDimensions = "dimensions"
	StepImages     = "images"
	StepVideo      = "video"
	StepResync     = "resync"

	StatusOK      = "ok"
	StatusSkipped = "skipped"
	StatusError   = "error"
)

type step struct {
	Step   string `json:"step"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// Start es el loop del worker: procesa las filas en cola (con `workers` en
// paralelo) hasta que ctx se cancela. Se corre bajo safe.Supervise.
func (s *Service) Start(ctx context.Context) {
	log.Printf("product bulk update worker: started (%d worker(s))", workers)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.loop(ctx, n == 0)
		}(i)
	}
	wg.Wait()
	log.Printf("product bulk update worker: stopped")
}

// loop procesa filas hasta que ctx se cancela. Solo un worker (requeueStale)
// recupera las filas que quedaron "running" de un proceso muerto.
func (s *Service) loop(ctx context.Context, requeueStale bool) {
	for ctx.Err() == nil {
		if requeueStale {
			if n, err := s.repo.RequeueStale(ctx, staleRunningAfter); err != nil {
				log.Printf("product bulk update worker: %v", err)
			} else if n > 0 {
				log.Printf("product bulk update worker: requeued %d stale item(s)", n)
			}
		}

		claim, err := s.claimNext(ctx)
		if err != nil {
			log.Printf("product bulk update worker: %v", err)
		}
		if claim == nil {
			select {
			case <-ctx.Done():
			case <-time.After(pollInterval):
			}
			continue
		}

		if !safe.Do("product-bulk-update-item", func() { s.process(ctx, claim) }) {
			s.finish(claim, mysqlInfra.ProductBulkUpdateOutcome{
				Status:       mysqlInfra.ProductBulkUpdateFailed,
				ErrorMessage: "Error interno durante la actualización (ver el log del servidor).",
				ActorID:      claim.ActorID,
			})
		}
	}
}

func (s *Service) claimNext(ctx context.Context) (*mysqlInfra.ProductBulkUpdateClaim, error) {
	var claim *mysqlInfra.ProductBulkUpdateClaim
	err := mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		claim, err = mysqlInfra.NewProductBulkUpdateRepository(tx).ClaimNext(ctx)
		return err
	})
	return claim, err
}

// process aplica una fila tomada de la cola y registra su resultado.
//
// Antes de modificar nada se valida lo que no depende de la fila: que el SKU
// exista, que sea de la source del lote y que la marca exista. Si algo de eso
// falla, el producto queda intacto. Después corren tres etapas independientes
// (datos generales + SEO + medidas, imágenes, video); cada una es idempotente,
// así que reintentar la fila completa es seguro.
func (s *Service) process(ctx context.Context, claim *mysqlInfra.ProductBulkUpdateClaim) {
	outcome := mysqlInfra.ProductBulkUpdateOutcome{ActorID: claim.ActorID}

	var payload rowPayload
	if err := json.Unmarshal(claim.Payload, &payload); err != nil {
		outcome.Status, outcome.ErrorMessage = mysqlInfra.ProductBulkUpdateFailed, "No se pudo leer lo que pedía la fila; sube el archivo de nuevo."
		s.finish(claim, outcome)
		return
	}

	product, err := s.productRepo.FindBySKU(ctx, claim.SKU)
	switch {
	case errors.Is(err, mysqlInfra.ErrProductNotFound):
		outcome.Status, outcome.ErrorMessage = mysqlInfra.ProductBulkUpdateMissing, "El SKU no existe en la tabla de productos."
		s.finish(claim, outcome)
		return
	case err != nil:
		outcome.Status, outcome.ErrorMessage = mysqlInfra.ProductBulkUpdateFailed, "Error al buscar el producto: "+err.Error()
		s.finish(claim, outcome)
		return
	case product.SourceID != claim.SourceID:
		outcome.Status = mysqlInfra.ProductBulkUpdateFailed
		outcome.ErrorMessage = "El SKU pertenece a otra source, no a la elegida al subir el archivo."
		s.finish(claim, outcome)
		return
	}

	var brandID *int64
	if payload.Brand != "" {
		brand, err := s.brandsRepo.FindByName(ctx, payload.Brand)
		switch {
		case errors.Is(err, mysqlInfra.ErrBrandNotFound):
			outcome.Status = mysqlInfra.ProductBulkUpdateFailed
			outcome.ErrorMessage = fmt.Sprintf("La marca %q no existe; créala en Configuración → Marcas y reintenta. No se modificó nada.", payload.Brand)
			s.finish(claim, outcome)
			return
		case err != nil:
			outcome.Status, outcome.ErrorMessage = mysqlInfra.ProductBulkUpdateFailed, "Error al buscar la marca: "+err.Error()
			s.finish(claim, outcome)
			return
		}
		brandID = &brand.ID
	}

	steps := s.updateProductData(ctx, product.ID, brandID, payload, claim.ActorID)
	steps = append(steps, s.replaceImages(ctx, product.ID, claim, payload))
	steps = append(steps, s.addVideo(ctx, product.ID, claim, payload))
	steps = append(steps, s.resync(ctx, product.ID, claim, steps))

	outcome.Steps, _ = json.Marshal(steps)
	var problems []string
	for _, st := range steps {
		if st.Status == StatusError {
			problems = append(problems, stepLabel[st.Step]+": "+st.Detail)
		}
	}
	if len(problems) == 0 {
		outcome.Status = mysqlInfra.ProductBulkUpdateDone
	} else {
		outcome.Status = mysqlInfra.ProductBulkUpdateFailed
		outcome.ErrorMessage = strings.Join(problems, " · ")
	}
	s.finish(claim, outcome)
}

var stepLabel = map[string]string{
	StepGeneral:    "Datos generales",
	StepSEO:        "SEO",
	StepDimensions: "Medidas",
	StepImages:     "Imágenes",
	StepVideo:      "Video",
	StepResync:     "Sincronización",
}

// resync vuelve a empujar el producto, ya actualizado en la base local, a las
// publicaciones que tenga en las conexiones elegidas al subir el archivo (el
// mismo flujo del botón "Resincronizar" del detalle del producto: manda todos
// los campos que el canal permite cambiar). Solo corre si todos los pasos
// anteriores salieron bien, para no publicar un estado a medias; si falla, la
// fila queda con error y al reintentarla se repite todo (cada paso es
// idempotente). Un producto sin publicación en una conexión no es un error.
func (s *Service) resync(ctx context.Context, productID int64, claim *mysqlInfra.ProductBulkUpdateClaim, previous []step) step {
	if len(claim.SyncConnectionIDs) == 0 {
		return step{Step: StepResync, Status: StatusSkipped}
	}
	for _, st := range previous {
		if st.Status == StatusError {
			return step{Step: StepResync, Status: StatusSkipped,
				Detail: "No se resincronizó porque hubo errores al guardar; corrígelos y reintenta."}
		}
	}

	s.resyncMu.Lock()
	defer s.resyncMu.Unlock()

	var parts, problems []string
	for _, connectionID := range claim.SyncConnectionIDs {
		name, err := s.listings.ValidateResyncConnection(ctx, connectionID)
		if err != nil {
			problems = append(problems, fmt.Sprintf("conexión %d: %v", connectionID, err))
			continue
		}

		result, err := s.listings.ResyncProductListings(ctx, channelListingsApp.ResyncListingsInput{
			ProductID:    productID,
			ConnectionID: connectionID,
		})
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", name, err))
			continue
		}

		updated, unchanged := 0, 0
		var failures []string
		for _, outcome := range result.Results {
			switch {
			case outcome.Error != "":
				failures = append(failures, outcome.Error)
			case outcome.Updated:
				updated++
			default:
				unchanged++
			}
		}
		switch {
		case len(failures) > 0:
			problems = append(problems, fmt.Sprintf("%s: %s", name, strings.Join(failures, "; ")))
		case len(result.Results) == 0:
			parts = append(parts, name+": sin publicación")
		case updated > 0:
			parts = append(parts, fmt.Sprintf("%s: %d publicación(es) actualizada(s)", name, updated))
		default:
			parts = append(parts, fmt.Sprintf("%s: sin cambios que enviar (%d)", name, unchanged))
		}
	}

	if len(problems) > 0 {
		return step{Step: StepResync, Status: StatusError, Detail: strings.Join(append(problems, parts...), " | ")}
	}
	return step{Step: StepResync, Status: StatusOK, Detail: strings.Join(parts, " | ")}
}

// updateProductData escribe, en UNA transacción, los datos generales del
// producto (nombre, marca, descripciones), su SEO (meta_title = título,
// meta_description = descripción corta) y sus medidas; si cualquiera falla, no
// se guarda ninguno de los tres.
func (s *Service) updateProductData(ctx context.Context, productID int64, brandID *int64, p rowPayload, actorID int64) []step {
	hasGeneral := p.Title != "" || p.ShortDescription != "" || p.LongDescription != "" || brandID != nil
	hasSEO := p.Title != "" || p.ShortDescription != ""
	hasDims := p.hasDimensions()

	if !hasGeneral && !hasDims {
		return []step{
			{Step: StepGeneral, Status: StatusSkipped},
			{Step: StepSEO, Status: StatusSkipped},
			{Step: StepDimensions, Status: StatusSkipped},
		}
	}

	err := mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
		if hasGeneral {
			fields := mysqlInfra.UpdateGeneralFields{BrandID: brandID}
			if p.Title != "" {
				fields.Name = &p.Title
			}
			if p.ShortDescription != "" {
				fields.ShortDescription = &p.ShortDescription
			}
			if p.LongDescription != "" {
				fields.Description = &p.LongDescription
			}
			if _, err := mysqlInfra.NewProductDetailsRepository(tx).UpdateGeneral(ctx, productID, actorID, fields); err != nil {
				return err
			}
		}
		if hasSEO {
			if err := upsertSEO(ctx, mysqlInfra.NewProductSEORepository(tx), productID, p, actorID); err != nil {
				return fmt.Errorf("error saving seo: %w", err)
			}
		}
		if hasDims {
			if err := upsertDimensions(ctx, mysqlInfra.NewProductDimensionsRepository(tx), productID, p, actorID); err != nil {
				return fmt.Errorf("error saving dimensions: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("product bulk update: product=%d: %v", productID, err)
		detail := "No se guardaron los datos generales, SEO ni medidas: " + err.Error()
		return []step{
			{Step: StepGeneral, Status: StatusError, Detail: detail},
			{Step: StepSEO, Status: StatusSkipped},
			{Step: StepDimensions, Status: StatusSkipped},
		}
	}

	general := step{Step: StepGeneral, Status: StatusSkipped}
	if hasGeneral {
		var changed []string
		if p.Title != "" {
			changed = append(changed, "nombre")
		}
		if brandID != nil {
			changed = append(changed, "marca")
		}
		if p.ShortDescription != "" {
			changed = append(changed, "descripción corta")
		}
		if p.LongDescription != "" {
			changed = append(changed, "descripción larga")
		}
		general = step{Step: StepGeneral, Status: StatusOK, Detail: "Actualizado: " + strings.Join(changed, ", ") + "."}
	}
	seo := step{Step: StepSEO, Status: StatusSkipped}
	if hasSEO {
		seo = step{Step: StepSEO, Status: StatusOK}
	}
	dims := step{Step: StepDimensions, Status: StatusSkipped}
	if hasDims {
		dims = step{Step: StepDimensions, Status: StatusOK}
	}
	return []step{general, seo, dims}
}

// upsertSEO crea el registro SEO del producto si no existe, o actualiza el
// existente; solo cambia meta_title y meta_description y respeta keywords.
func upsertSEO(ctx context.Context, repo *mysqlInfra.ProductSEORepository, productID int64, p rowPayload, actorID int64) error {
	var title, description *string
	if p.Title != "" {
		title = &p.Title
	}
	if p.ShortDescription != "" {
		description = &p.ShortDescription
	}

	existing, err := repo.FindByProductID(ctx, productID)
	if errors.Is(err, mysqlInfra.ErrProductSEONotFound) {
		_, err = repo.Create(ctx, mysqlInfra.CreateProductSEOInput{
			ProductID:       productID,
			MetaTitle:       title,
			MetaDescription: description,
			CreatedBy:       actorID,
		})
		return err
	}
	if err != nil {
		return err
	}
	if title == nil {
		title = existing.MetaTitle
	}
	if description == nil {
		description = existing.MetaDescription
	}
	_, err = repo.Update(ctx, productID, mysqlInfra.UpdateProductSEOInput{
		MetaTitle:       title,
		MetaDescription: description,
		Keywords:        existing.Keywords,
		UpdatedBy:       actorID,
	})
	return err
}

// upsertDimensions actualiza solo las medidas presentes en la fila y conserva
// las demás. Si el producto aún no tiene medidas, las que falten toman el mismo
// valor por omisión que el alta masiva de dimensiones (1; el diámetro, el
// ancho). El volumen lo recalcula el repositorio.
func upsertDimensions(ctx context.Context, repo *mysqlInfra.ProductDimensionsRepository, productID int64, p rowPayload, actorID int64) error {
	pick := func(value *float64, current string) string {
		if value != nil {
			return strconv.FormatFloat(*value, 'f', 2, 64)
		}
		return current
	}

	existing, err := repo.FindByProductID(ctx, productID)
	if errors.Is(err, mysqlInfra.ErrProductDimensionsNotFound) {
		width := pick(p.Width, "1.00")
		diameter := pick(p.Diameter, width)
		_, err = repo.Create(ctx, mysqlInfra.CreateProductDimensionsInput{
			ProductID: productID,
			Weight:    pick(p.Weight, "1.00"),
			Length:    pick(p.Length, "1.00"),
			Width:     width,
			Height:    pick(p.Height, "1.00"),
			Diameter:  diameter,
			CreatedBy: actorID,
		})
		return err
	}
	if err != nil {
		return err
	}
	_, err = repo.Update(ctx, productID, mysqlInfra.UpdateProductDimensionsInput{
		Weight:    pick(p.Weight, existing.Weight),
		Length:    pick(p.Length, existing.Length),
		Width:     pick(p.Width, existing.Width),
		Height:    pick(p.Height, existing.Height),
		Diameter:  pick(p.Diameter, existing.Diameter),
		UpdatedBy: actorID,
	})
	return err
}

// replaceImages deja la galería del producto exactamente como la fila la pide
// (foto_1…foto_7 y, al final, foto_aplicacion). Si alguna URL no responde no se
// cambia ninguna imagen.
func (s *Service) replaceImages(ctx context.Context, productID int64, claim *mysqlInfra.ProductBulkUpdateClaim, p rowPayload) step {
	if len(p.Photos) == 0 {
		return step{Step: StepImages, Status: StatusSkipped}
	}

	result, err := s.media.ReplaceImagesForProduct(ctx, productImageImportApp.ReplaceImagesInput{
		ProductID:     productID,
		StorageDiskID: claim.StorageDiskID,
		URLs:          p.Photos,
		ActorID:       claim.ActorID,
	})
	if err != nil {
		if errors.Is(err, productImageImportApp.ErrInvalidImageURLs) && result != nil {
			var bad []string
			for _, r := range result.Results {
				if r.Error != "" {
					bad = append(bad, fmt.Sprintf("%s (%s)", r.URL, r.Error))
				}
			}
			return step{Step: StepImages, Status: StatusError,
				Detail: "No se cambió ninguna imagen; estas URLs fallaron: " + strings.Join(bad, "; ")}
		}
		return step{Step: StepImages, Status: StatusError, Detail: err.Error()}
	}
	return step{Step: StepImages, Status: StatusOK,
		Detail: fmt.Sprintf("%d imagen(es); reemplazaron a las anteriores.", len(result.Results))}
}

// addVideo AGREGA el video (archivo o enlace de YouTube) a los que el producto
// ya tenga; si el mismo enlace ya estaba vinculado, no lo duplica.
func (s *Service) addVideo(ctx context.Context, productID int64, claim *mysqlInfra.ProductBulkUpdateClaim, p rowPayload) step {
	if p.Video == "" {
		return step{Step: StepVideo, Status: StatusSkipped}
	}

	result, err := s.media.ImportVideosForProduct(ctx, productImageImportApp.ImportVideosForProductInput{
		ProductID:     productID,
		StorageDiskID: claim.StorageDiskID,
		URLs:          []string{p.Video},
		ActorID:       claim.ActorID,
	})
	if err != nil {
		return step{Step: StepVideo, Status: StatusError, Detail: err.Error()}
	}
	if len(result.Results) == 0 || !result.Results[0].Success {
		reason := "no se pudo agregar"
		if len(result.Results) > 0 && result.Results[0].Error != "" {
			reason = result.Results[0].Error
		}
		return step{Step: StepVideo, Status: StatusError, Detail: p.Video + ": " + reason}
	}
	return step{Step: StepVideo, Status: StatusOK, Detail: "Video agregado: " + result.Results[0].URL}
}

// finish guarda el resultado con un contexto nuevo: el del worker puede estar
// cancelado por un apagado, y el resultado de una fila que ya se aplicó no debe
// perderse.
func (s *Service) finish(claim *mysqlInfra.ProductBulkUpdateClaim, outcome mysqlInfra.ProductBulkUpdateOutcome) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.repo.SetOutcome(ctx, claim.ID, outcome); err != nil {
		log.Printf("product bulk update worker: %v", err)
		return
	}
	log.Printf("product bulk update worker: item=%d sku=%s status=%s", claim.ID, claim.SKU, outcome.Status)
}
