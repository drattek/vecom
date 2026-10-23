package meli_bulk_import

import (
	"bytes"
	"errors"
	"testing"

	"github.com/xuri/excelize/v2"
)

func sheetFile(t *testing.T, rows [][]any) *bytes.Reader {
	t.Helper()
	book := excelize.NewFile()
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := book.SetSheetRow("Sheet1", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := book.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buf.Bytes())
}

func TestParseSheet(t *testing.T) {
	rows, err := parseSheet(sheetFile(t, [][]any{
		{"MELI_ID", "SKU"},
		{"MLM123456789", " 84910Y02G1 "},
		{nil, nil},
		{"", "SIN-ID"},
		{"1987654321", "ABC"},
	}))
	if err != nil {
		t.Fatalf("parseSheet: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("esperaba 3 filas (se salta la vacía), obtuve %d: %+v", len(rows), rows)
	}
	if rows[0].sku != "84910Y02G1" || rows[0].meliID != "MLM123456789" || rows[0].number != 2 {
		t.Errorf("primera fila mal leída: %+v", rows[0])
	}
	if rows[1].sku != "SIN-ID" || rows[1].meliID != "" {
		t.Errorf("fila sin meli_id debía conservarse para reportarla: %+v", rows[1])
	}
}

func TestParseSheetErrors(t *testing.T) {
	if _, err := parseSheet(bytes.NewReader([]byte("no es excel"))); !errors.Is(err, ErrInvalidFile) {
		t.Errorf("archivo inválido: %v", err)
	}
	if _, err := parseSheet(sheetFile(t, [][]any{{"codigo", "publicacion"}, {"A", "B"}})); !errors.Is(err, ErrMissingColumns) {
		t.Errorf("sin columnas: %v", err)
	}
	if _, err := parseSheet(sheetFile(t, [][]any{{"sku", "meli_id"}})); !errors.Is(err, ErrEmptyFile) {
		t.Errorf("sin filas: %v", err)
	}
}
