package notifications

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// bulkWindow: cuánto después de terminar todavía se avisa de un lote.
const bulkWindow = 2 * time.Hour

type batchSource interface {
	FinishedBatches(ctx context.Context, kind string, window time.Duration) ([]mysqlInfra.FinishedBatchDTO, error)
	FailedBatchRows(ctx context.Context, kind string, batchID int64) ([]mysqlInfra.FailedBatchRowDTO, error)
}

var bulkKindLabels = map[string]string{
	mysqlInfra.BulkKindMeliImport:    "importación de publicaciones de MercadoLibre",
	mysqlInfra.BulkKindProductUpdate: "actualización masiva de productos",
}

var bulkStatusLabels = map[string]string{
	"failed":    "con error",
	"missing":   "faltante",
	"duplicate": "duplicado",
}

// checkFinishedBatches avisa a quien inició cada lote que ya terminó. Se
// detecta leyendo la base, así no se pierde con un reinicio; la dedupe_key
// evita repetirlo y cambia si se reintentan filas.
func (s *Service) checkFinishedBatches(ctx context.Context) error {
	for _, kind := range []string{mysqlInfra.BulkKindMeliImport, mysqlInfra.BulkKindProductUpdate} {
		batches, err := s.batches.FinishedBatches(ctx, kind, bulkWindow)
		if err != nil {
			return err
		}
		for _, batch := range batches {
			if err := s.notifyBatch(ctx, batch); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) notifyBatch(ctx context.Context, batch mysqlInfra.FinishedBatchDTO) error {
	dedupeKey := "bulk:" + batch.Kind + ":" + strconv.FormatInt(batch.ID, 10) + ":" + strconv.FormatInt(batch.FinishedAtUnix, 10)

	return s.inUnit(ctx, func(tx unit) error {
		n := notice{notificationType: TypeBulkProcessFinished, onlyUserID: batch.CreatedBy}

		// Se evita armar el archivo si no hay a quién enviarlo o ya se avisó.
		users, err := s.audience(ctx, tx, n)
		if err != nil || len(users) == 0 {
			return err
		}
		delivered, err := tx.Delivered(ctx, batch.CreatedBy, dedupeKey)
		if err != nil || delivered {
			return err
		}

		icon := "✅"
		if batch.Done < batch.Total {
			icon = "🟠"
		}
		var text strings.Builder
		fmt.Fprintf(&text, "%s Terminó tu %s.\nArchivo: %s\nFilas: %d · Correctas: %d",
			icon, bulkKindLabels[batch.Kind], batch.FileName, batch.Total, batch.Done)
		for _, part := range []struct {
			label string
			count int
		}{{"Con error", batch.Failed}, {"Faltantes", batch.Missing}, {"Duplicadas", batch.Duplicate}} {
			if part.count > 0 {
				fmt.Fprintf(&text, " · %s: %d", part.label, part.count)
			}
		}
		n.text = text.String()

		if batch.Done < batch.Total {
			rows, err := s.batches.FailedBatchRows(ctx, batch.Kind, batch.ID)
			if err != nil {
				return err
			}
			cells := make([][]string, 0, len(rows))
			for _, row := range rows {
				status := bulkStatusLabels[row.Status]
				if status == "" {
					status = row.Status
				}
				cells = append(cells, []string{strconv.Itoa(row.SheetRow), row.SKU, status, sanitize(row.Error)})
			}
			if len(cells) > 0 {
				n.attachmentName = fmt.Sprintf("lote-%d-pendientes.csv", batch.ID)
				n.attachment = buildCSV([]string{"fila", "sku", "estado", "detalle"}, cells)
			}
		}

		return s.enqueue(ctx, tx, dedupeKey, n)
	})
}
