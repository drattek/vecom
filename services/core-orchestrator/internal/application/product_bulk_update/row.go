package product_bulk_update

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	neturl "net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// Columnas del Excel (en este orden en la plantilla). La cabecera se compara sin
// distinguir mayúsculas, acentos ni espacios/guiones (ver normalizeHeader).
const (
	colSKU              = "sku"
	colBrand            = "marca"
	colTitle            = "titulo_optimizado"
	colShortDescription = "descripcion_corta"
	colLongDescription  = "descripcion_larga"
	colApplicationPhoto = "foto_aplicacion"
	colVideo            = "video"
	colWidth            = "ancho_cm"
	colHeight           = "alto_cm"
	colLength           = "largo_cm"
	colDiameter         = "diametro"
	colWeight           = "peso_kg"
)

// maxPhotos es el número de columnas foto_N (foto_1 … foto_7).
const maxPhotos = 7

func photoColumn(n int) string { return "foto_" + strconv.Itoa(n) }

// templateColumns es el orden de las columnas de la plantilla descargable.
func templateColumns() []string {
	cols := []string{colSKU, colBrand, colTitle, colShortDescription, colLongDescription}
	for n := 1; n <= maxPhotos; n++ {
		cols = append(cols, photoColumn(n))
	}
	return append(cols, colApplicationPhoto, colVideo, colWidth, colHeight, colLength, colDiameter, colWeight)
}

// Límites de las columnas de ecom_products / ecom_product_seo.
const (
	maxTitleRunes       = 255 // ecom_products.name, ecom_product_seo.meta_title
	maxShortDescRunes   = 255 // ecom_products.short_description, meta_description
	maxLongDescBytes    = 60000
	maxBrandRunes       = 255
	maxMeasure          = 99999999.0
	maxRows             = 5000
	maxFieldDetailRunes = 300
)

var (
	ErrInvalidFile    = errors.New("the file is not a valid Excel (.xlsx) file")
	ErrMissingColumns = errors.New("the Excel must have the column sku in its first row")
	ErrNoDataColumns  = errors.New("the Excel has no data columns besides sku")
	ErrEmptyFile      = errors.New("the Excel has no rows to update")
	ErrTooManyRows    = fmt.Errorf("the Excel has more than %d rows", maxRows)
)

// rowPayload es lo que una fila del Excel pide cambiar; un valor vacío/nil
// significa "no tocar ese campo". Se guarda como JSON en el ítem para que el
// worker y los reintentos no dependan del archivo.
type rowPayload struct {
	Brand            string `json:"brand,omitempty"`
	Title            string `json:"title,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
	LongDescription  string `json:"longDescription,omitempty"`
	// Photos es la galería en su orden final: foto_1…foto_7 (las que existan) y
	// al final foto_aplicacion, si la hay.
	Photos   []string `json:"photos,omitempty"`
	Video    string   `json:"video,omitempty"`
	Width    *float64 `json:"width,omitempty"`
	Height   *float64 `json:"height,omitempty"`
	Length   *float64 `json:"length,omitempty"`
	Diameter *float64 `json:"diameter,omitempty"`
	Weight   *float64 `json:"weight,omitempty"`
}

func (p rowPayload) hasDimensions() bool {
	return p.Width != nil || p.Height != nil || p.Length != nil || p.Diameter != nil || p.Weight != nil
}

func (p rowPayload) isEmpty() bool {
	return p.Brand == "" && p.Title == "" && p.ShortDescription == "" && p.LongDescription == "" &&
		len(p.Photos) == 0 && p.Video == "" && !p.hasDimensions()
}

// sheetRow es una fila del Excel ya leída: su número (1-based, el de Excel) y
// el valor de cada columna conocida por nombre normalizado.
type sheetRow struct {
	number int
	cells  map[string]string
}

var headerReplacer = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	" ", "_", "-", "_",
)

func normalizeHeader(s string) string {
	return headerReplacer.Replace(strings.ToLower(strings.TrimSpace(s)))
}

// parseSheet lee la primera hoja: la fila de cabecera es la primera que contiene
// la columna sku (cualquier orden de columnas); las filas totalmente vacías se
// omiten y las columnas que no son de la plantilla se ignoran.
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

	known := map[string]bool{}
	for _, col := range templateColumns() {
		known[col] = true
	}

	var columns map[int]string // índice de columna → nombre normalizado
	headerRow := -1
	for i, row := range all {
		candidate := map[int]string{}
		hasSKU := false
		for c, cell := range row {
			name := normalizeHeader(cell)
			if !known[name] {
				continue
			}
			candidate[c] = name
			if name == colSKU {
				hasSKU = true
			}
		}
		if hasSKU {
			columns, headerRow = candidate, i
			break
		}
	}
	if headerRow < 0 {
		return nil, ErrMissingColumns
	}
	if len(columns) < 2 {
		return nil, ErrNoDataColumns
	}

	var rows []sheetRow
	for i := headerRow + 1; i < len(all); i++ {
		cells := map[string]string{}
		empty := true
		for c, name := range columns {
			if c < len(all[i]) {
				if v := strings.TrimSpace(all[i][c]); v != "" {
					cells[name] = v
					empty = false
				}
			}
		}
		if empty {
			continue
		}
		rows = append(rows, sheetRow{number: i + 1, cells: cells})
		if len(rows) > maxRows {
			return nil, ErrTooManyRows
		}
	}
	if len(rows) == 0 {
		return nil, ErrEmptyFile
	}
	return rows, nil
}

// buildPayload valida una fila y arma lo que pide cambiar. El error es un
// mensaje para el usuario (en español) y deja la fila como "failed".
func buildPayload(cells map[string]string) (rowPayload, error) {
	p := rowPayload{
		Brand:            cells[colBrand],
		Title:            cells[colTitle],
		ShortDescription: cells[colShortDescription],
		LongDescription:  cells[colLongDescription],
		Video:            cells[colVideo],
	}

	if utf8.RuneCountInString(p.Brand) > maxBrandRunes {
		return p, fmt.Errorf("La marca supera los %d caracteres.", maxBrandRunes)
	}
	if utf8.RuneCountInString(p.Title) > maxTitleRunes {
		return p, fmt.Errorf("titulo_optimizado supera los %d caracteres (tiene %d).", maxTitleRunes, utf8.RuneCountInString(p.Title))
	}
	if utf8.RuneCountInString(p.ShortDescription) > maxShortDescRunes {
		return p, fmt.Errorf("descripcion_corta supera los %d caracteres (tiene %d).", maxShortDescRunes, utf8.RuneCountInString(p.ShortDescription))
	}
	if len(p.LongDescription) > maxLongDescBytes {
		return p, errors.New("descripcion_larga es demasiado larga.")
	}

	// Galería: foto_1…foto_7 en orden (las vacías se saltan) y, siempre al final,
	// foto_aplicacion. Una URL repetida cuenta una sola vez.
	seen := map[string]bool{}
	addPhoto := func(column string) error {
		value := cells[column]
		if value == "" || seen[value] {
			return nil
		}
		if err := validateURL(value); err != nil {
			return fmt.Errorf("%s: %v", column, err)
		}
		seen[value] = true
		p.Photos = append(p.Photos, value)
		return nil
	}
	for n := 1; n <= maxPhotos; n++ {
		if err := addPhoto(photoColumn(n)); err != nil {
			return p, err
		}
	}
	if err := addPhoto(colApplicationPhoto); err != nil {
		return p, err
	}

	if p.Video != "" {
		if err := validateURL(p.Video); err != nil {
			return p, fmt.Errorf("video: %v", err)
		}
	}

	var err error
	for _, m := range []struct {
		column string
		target **float64
	}{
		{colWidth, &p.Width}, {colHeight, &p.Height}, {colLength, &p.Length},
		{colDiameter, &p.Diameter}, {colWeight, &p.Weight},
	} {
		if *m.target, err = parseMeasure(cells[m.column]); err != nil {
			return p, fmt.Errorf("%s: %v", m.column, err)
		}
	}

	if p.isEmpty() {
		return p, errors.New("La fila no tiene ningún dato para actualizar además del SKU.")
	}
	return p, nil
}

func validateURL(raw string) error {
	parsed, err := neturl.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%q no es una URL válida (debe empezar con http:// o https://)", truncate(raw))
	}
	return nil
}

// parseMeasure convierte una celda de medida. Una celda vacía es "no tocar"
// (nil). Acepta coma o punto decimal ("1,5" / "1.5") y rechaza texto, cero y
// negativos.
func parseMeasure(raw string) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	normalized := raw
	switch {
	case strings.Contains(raw, ",") && strings.Contains(raw, "."):
		normalized = strings.ReplaceAll(raw, ",", "") // 1,234.5
	case strings.Contains(raw, ","):
		normalized = strings.ReplaceAll(raw, ",", ".") // 1,5
	}
	value, err := strconv.ParseFloat(normalized, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, fmt.Errorf("%q no es un número", truncate(raw))
	}
	if value <= 0 {
		return nil, errors.New("debe ser mayor que 0")
	}
	if value > maxMeasure {
		return nil, errors.New("el valor es demasiado grande")
	}
	return &value, nil
}

func truncate(s string) string {
	if utf8.RuneCountInString(s) <= maxFieldDetailRunes {
		return s
	}
	return string([]rune(s)[:maxFieldDetailRunes]) + "…"
}
