package product_bulk_update

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func workbook(t *testing.T, rows [][]any) *bytes.Reader {
	t.Helper()
	book := excelize.NewFile()
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := book.SetSheetRow("Sheet1", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	buf, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buf.Bytes())
}

func TestParseSheetReadsKnownColumnsAnyOrderAndAccents(t *testing.T) {
	rows, err := parseSheet(workbook(t, [][]any{
		{"Peso_KG", "SKU", "Título optimizado", "ignorada", "foto 1"},
		{"2,5", "A-1", "Filtro de aceite", "x", "https://cdn/x.jpg"},
		{nil, nil, nil, nil, nil},
		{"", "B-2", "", "", ""},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (empty rows are skipped)", len(rows))
	}
	if rows[0].number != 2 || rows[0].cells[colSKU] != "A-1" || rows[0].cells[colTitle] != "Filtro de aceite" ||
		rows[0].cells[colWeight] != "2,5" || rows[0].cells[photoColumn(1)] != "https://cdn/x.jpg" {
		t.Fatalf("unexpected row: %+v", rows[0])
	}
	if _, ok := rows[0].cells["ignorada"]; ok {
		t.Fatal("unknown columns must be ignored")
	}
	if rows[1].number != 4 {
		t.Fatalf("row number = %d, want 4", rows[1].number)
	}
}

func TestParseSheetErrors(t *testing.T) {
	cases := []struct {
		name string
		rows [][]any
		want error
	}{
		{"no sku column", [][]any{{"marca", "titulo_optimizado"}, {"a", "b"}}, ErrMissingColumns},
		{"only sku", [][]any{{"sku"}, {"A-1"}}, ErrNoDataColumns},
		{"no rows", [][]any{{"sku", "marca"}}, ErrEmptyFile},
	}
	for _, tc := range cases {
		if _, err := parseSheet(workbook(t, tc.rows)); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
	if _, err := parseSheet(strings.NewReader("not an excel")); !errors.Is(err, ErrInvalidFile) {
		t.Errorf("garbage: err = %v, want ErrInvalidFile", err)
	}
}

func TestBuildPayloadGalleryOrderPutsApplicationPhotoLast(t *testing.T) {
	p, err := buildPayload(map[string]string{
		colApplicationPhoto: "https://cdn/app.jpg",
		photoColumn(3):      "https://cdn/3.jpg",
		photoColumn(1):      "https://cdn/1.jpg",
		photoColumn(2):      "https://cdn/1.jpg", // repetida: cuenta una vez
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://cdn/1.jpg", "https://cdn/3.jpg", "https://cdn/app.jpg"}
	if len(p.Photos) != len(want) {
		t.Fatalf("photos = %v, want %v", p.Photos, want)
	}
	for i := range want {
		if p.Photos[i] != want[i] {
			t.Fatalf("photos = %v, want %v", p.Photos, want)
		}
	}
}

func TestBuildPayloadWithoutApplicationPhoto(t *testing.T) {
	p, err := buildPayload(map[string]string{photoColumn(1): "https://cdn/1.jpg"})
	if err != nil || len(p.Photos) != 1 {
		t.Fatalf("photos = %v, err = %v", p.Photos, err)
	}
}

func TestBuildPayloadMeasures(t *testing.T) {
	p, err := buildPayload(map[string]string{colWeight: "1,5", colWidth: "1,234.5", colLength: "10"})
	if err != nil {
		t.Fatal(err)
	}
	if *p.Weight != 1.5 || *p.Width != 1234.5 || *p.Length != 10 {
		t.Fatalf("got weight=%v width=%v length=%v", *p.Weight, *p.Width, *p.Length)
	}
	if p.Height != nil || p.Diameter != nil {
		t.Fatal("empty measures must stay nil (not touched)")
	}
}

func TestBuildPayloadRejectsBadValues(t *testing.T) {
	cases := map[string]map[string]string{
		"text measure":     {colWeight: "pesado"},
		"zero measure":     {colWidth: "0"},
		"negative measure": {colHeight: "-3"},
		"bad photo url":    {photoColumn(1): "foto.jpg"},
		"bad video url":    {colVideo: "ftp://x/y.mp4"},
		"long title":       {colTitle: strings.Repeat("a", maxTitleRunes+1)},
		"long short desc":  {colShortDescription: strings.Repeat("a", maxShortDescRunes+1)},
		"nothing to do":    {},
	}
	for name, cells := range cases {
		if _, err := buildPayload(cells); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
