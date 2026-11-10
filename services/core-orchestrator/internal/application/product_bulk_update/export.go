package product_bulk_update

import (
	"context"
	"encoding/json"
	"fmt"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"

	"github.com/xuri/excelize/v2"
)

var exportHeaders = []string{
	"Fila", "SKU", "Producto", "Estado", "Datos generales", "SEO", "Medidas", "Imágenes", "Video", "Sincronización", "Detalle",
}

var statusLabel = map[string]string{
	mysqlInfra.ProductBulkUpdatePending: "Pendiente",
	mysqlInfra.ProductBulkUpdateRunning: "Procesando",
	mysqlInfra.ProductBulkUpdateDone:    "Actualizado",
	mysqlInfra.ProductBulkUpdateFailed:  "Con error",
	mysqlInfra.ProductBulkUpdateMissing: "SKU inexistente",
}

// ExportBatch arma un Excel con una fila por fila del archivo del lote: su
// estado final, el resultado de cada paso (datos generales, SEO, medidas,
// imágenes, video) y el detalle del error, para corregir y volver a subir lo
// que falló.
func (s *Service) ExportBatch(ctx context.Context, batchID int64) (fileName string, content []byte, err error) {
	batch, err := s.repo.FindBatch(ctx, batchID)
	if err != nil {
		return "", nil, err
	}
	items, err := s.repo.ListByBatch(ctx, batchID)
	if err != nil {
		return "", nil, err
	}

	book := excelize.NewFile()
	defer book.Close()
	const sheet = "Lote"
	if err := book.SetSheetName(book.GetSheetName(0), sheet); err != nil {
		return "", nil, fmt.Errorf("error building the export sheet: %w", err)
	}
	if err := setRow(book, sheet, 1, stringsToAny(exportHeaders)); err != nil {
		return "", nil, err
	}

	for i, item := range items {
		byStep := map[string]step{}
		var parsed []step
		if len(item.Steps) > 0 && json.Unmarshal(item.Steps, &parsed) == nil {
			for _, st := range parsed {
				byStep[st.Step] = st
			}
		}
		stepCell := func(name string) string {
			st, ok := byStep[name]
			switch {
			case !ok:
				return ""
			case st.Status == StatusSkipped:
				return "Sin cambios"
			case st.Status == StatusError:
				return "Error"
			default:
				return "Actualizado"
			}
		}

		productName, detail := "", ""
		if item.ProductName != nil {
			productName = *item.ProductName
		}
		if item.ErrorMessage != nil {
			detail = *item.ErrorMessage
		}
		row := []any{
			item.SheetRow, item.SKU, productName, statusLabel[item.Status],
			stepCell(StepGeneral), stepCell(StepSEO), stepCell(StepDimensions), stepCell(StepImages), stepCell(StepVideo), stepCell(StepResync),
			detail,
		}
		if err := setRow(book, sheet, i+2, row); err != nil {
			return "", nil, err
		}
	}

	buffer, err := book.WriteToBuffer()
	if err != nil {
		return "", nil, fmt.Errorf("error writing the export file: %w", err)
	}
	return batch.FileName, buffer.Bytes(), nil
}

// Template genera la plantilla descargable: la hoja "Productos" con las
// columnas que lee el importador (la primera hoja es la que se procesa) y una
// hoja "Instrucciones" con las reglas de cada columna.
func (s *Service) Template() ([]byte, error) {
	book := excelize.NewFile()
	defer book.Close()

	const sheet = "Productos"
	if err := book.SetSheetName(book.GetSheetName(0), sheet); err != nil {
		return nil, fmt.Errorf("error building the template: %w", err)
	}
	if err := setRow(book, sheet, 1, stringsToAny(templateColumns())); err != nil {
		return nil, err
	}

	const help = "Instrucciones"
	if _, err := book.NewSheet(help); err != nil {
		return nil, fmt.Errorf("error building the template: %w", err)
	}
	lines := [][]any{
		{"Columna", "Qué hace"},
		{colSKU, "Obligatoria. Debe coincidir exactamente con el SKU del producto y pertenecer a la source elegida al subir el archivo."},
		{colBrand, "Nombre de una marca que ya exista (no distingue mayúsculas). Si no existe, la fila falla y no se modifica nada."},
		{colTitle, "Reemplaza el nombre del producto y se guarda también como meta título (SEO). Máximo 255 caracteres."},
		{colShortDescription, "Se guarda como descripción corta del producto y como meta descripción (SEO). Máximo 255 caracteres."},
		{colLongDescription, "Se guarda solo como descripción del producto."},
		{"foto_1 … foto_7", "URLs de las fotos de la galería, en orden; foto_1 es la portada. Puedes llenar solo algunas. Si llenas al menos una (o foto_aplicacion), REEMPLAZAN todas las imágenes actuales del producto."},
		{colApplicationPhoto, "URL de la foto de aplicación; siempre queda al final de la galería. Opcional."},
		{colVideo, "URL de un video: un enlace de YouTube o un archivo de video. Se AGREGA a los videos que ya tenga el producto."},
		{"ancho_cm, alto_cm, largo_cm, diametro, peso_kg", "Medidas en cm y kg (acepta 1.5 o 1,5). Las que dejes vacías no se modifican; el volumen se recalcula solo."},
		{"", ""},
		{"Celdas vacías", "Una celda vacía significa «no tocar ese dato»: nunca borra información."},
	}
	for i, line := range lines {
		if err := setRow(book, help, i+1, line); err != nil {
			return nil, err
		}
	}
	book.SetActiveSheet(0)

	buffer, err := book.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("error writing the template: %w", err)
	}
	return buffer.Bytes(), nil
}

func setRow(book *excelize.File, sheet string, rowNumber int, values []any) error {
	cell, err := excelize.CoordinatesToCellName(1, rowNumber)
	if err != nil {
		return fmt.Errorf("error building the sheet: %w", err)
	}
	if err := book.SetSheetRow(sheet, cell, &values); err != nil {
		return fmt.Errorf("error building the sheet: %w", err)
	}
	return nil
}

func stringsToAny(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}
