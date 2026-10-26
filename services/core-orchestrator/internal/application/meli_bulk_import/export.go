package meli_bulk_import

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"

	"github.com/xuri/excelize/v2"
)

// priceModerationTag is the MercadoLibre tag that marks a listing penalized by
// moderation for an unusual price change (typically the jump from the
// 999,999 placeholder to the real price pushed by the import).
const priceModerationTag = "moderation_penalty"

var exportHeaders = []string{
	"Fila", "SKU", "ID de MercadoLibre", "ID de categoría", "Estado de vinculación",
	"Precio/stock actualizado", "Detalle precio/stock", "Estado en MercadoLibre",
	"Subestado en MercadoLibre", "Moderación de precio", "Etiquetas de MercadoLibre",
}

type exportStep struct {
	Step   string `json:"step"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// ExportBatch builds an Excel with one row per row of the batch's file: its SKU,
// MercadoLibre id and category, whether it ended up linked, whether price/stock
// were pushed and whether MercadoLibre currently flags the listing with price
// moderation. The listing state is read live from MercadoLibre (not from what was
// recorded at import time), since moderation happens after the import.
func (s *Service) ExportBatch(ctx context.Context, batchID int64) (fileName string, content []byte, err error) {
	batch, err := s.repo.FindBatch(ctx, batchID)
	if err != nil {
		return "", nil, err
	}
	items, err := s.repo.ListByBatch(ctx, batchID)
	if err != nil {
		return "", nil, err
	}

	var externalIDs []string
	seen := map[string]bool{}
	for _, item := range items {
		if id := derefOr(item.ExternalID); id != "" && !seen[id] {
			seen[id] = true
			externalIDs = append(externalIDs, id)
		}
	}
	live := map[string]mercadoLibreInfra.ItemStatus{}
	if len(externalIDs) > 0 {
		accessToken, err := s.tokenService.EnsureValidAccessToken(ctx, batch.ConnectionID)
		if err != nil {
			return "", nil, fmt.Errorf("error getting mercadolibre access token: %w", err)
		}
		statuses, err := s.itemsHandler.GetItemsStatus(ctx, accessToken, externalIDs)
		if err != nil {
			return "", nil, fmt.Errorf("error reading the listings from mercadolibre: %w", err)
		}
		for _, status := range statuses {
			live[status.ID] = status
		}
	}

	book := excelize.NewFile()
	defer book.Close()
	const sheet = "Lote"
	if err := book.SetSheetName(book.GetSheetName(0), sheet); err != nil {
		return "", nil, fmt.Errorf("error building the export sheet: %w", err)
	}
	headers := make([]any, len(exportHeaders))
	for i, h := range exportHeaders {
		headers[i] = h
	}
	if err := book.SetSheetRow(sheet, "A1", &headers); err != nil {
		return "", nil, fmt.Errorf("error building the export sheet: %w", err)
	}

	for i, item := range items {
		externalID := derefOr(item.ExternalID)
		priceStock, priceStockDetail := priceStockState(item)

		category, meliStatus, subStatus, moderation, tags := "", "", "", "", ""
		if externalID != "" {
			if status, ok := live[externalID]; ok {
				category = status.CategoryID
				meliStatus = status.Status
				subStatus = strings.Join(status.SubStatus, ", ")
				tags = strings.Join(status.Tags, ", ")
				moderation = "No"
				for _, tag := range status.Tags {
					if tag == priceModerationTag {
						moderation = "Sí"
						break
					}
				}
			} else {
				meliStatus = "No encontrada en MercadoLibre"
			}
		}

		row := []any{item.SheetRow, item.SKU, externalID, category, linkState(item),
			priceStock, priceStockDetail, meliStatus, subStatus, moderation, tags}
		cell, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return "", nil, fmt.Errorf("error building the export sheet: %w", err)
		}
		if err := book.SetSheetRow(sheet, cell, &row); err != nil {
			return "", nil, fmt.Errorf("error building the export sheet: %w", err)
		}
	}

	buffer, err := book.WriteToBuffer()
	if err != nil {
		return "", nil, fmt.Errorf("error writing the export file: %w", err)
	}
	return batch.FileName, buffer.Bytes(), nil
}

func derefOr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func itemSteps(item mysqlInfra.MeliImportItemDTO) map[string]exportStep {
	steps := map[string]exportStep{}
	if len(item.Steps) == 0 {
		return steps
	}
	var parsed []exportStep
	if err := json.Unmarshal(item.Steps, &parsed); err != nil {
		return steps
	}
	for _, step := range parsed {
		steps[step.Step] = step
	}
	return steps
}

// linkState describes whether the row's listing ended up linked to its product.
func linkState(item mysqlInfra.MeliImportItemDTO) string {
	switch item.Status {
	case mysqlInfra.MeliImportPending, mysqlInfra.MeliImportRunning:
		return "Pendiente de importar"
	case mysqlInfra.MeliImportMissing:
		return "No vinculada: el SKU no existe"
	case mysqlInfra.MeliImportDuplicate:
		return "No vinculada: el producto ya tenía una publicación"
	case mysqlInfra.MeliImportFailed:
		return "No vinculada: " + derefOr(item.ErrorMessage)
	}
	step, ok := itemSteps(item)["link"]
	switch {
	case ok && step.Status == "ok":
		return "Vinculada"
	case ok && step.Detail != "":
		return "No vinculada: " + step.Detail
	default:
		return "No vinculada"
	}
}

// priceStockState reports whether the import pushed price and stock, with the
// recorded detail (what was sent, or why nothing was).
func priceStockState(item mysqlInfra.MeliImportItemDTO) (string, string) {
	step, ok := itemSteps(item)["price_stock"]
	switch {
	case !ok:
		return "No", ""
	case step.Status == "ok":
		return "Sí", step.Detail
	case step.Status == "error":
		return "Error", step.Detail
	default:
		return "No", step.Detail
	}
}
