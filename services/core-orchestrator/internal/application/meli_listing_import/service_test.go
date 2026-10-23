package meli_listing_import

import (
	"errors"
	"testing"

	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
)

func TestNormalizeItemID(t *testing.T) {
	valid := map[string]string{
		"MLM123456789":  "MLM123456789",
		" mlm123456789": "MLM123456789",
		"MLM-123456789": "MLM123456789",
		"123456789":     "MLM123456789",
		"https://articulo.mercadolibre.com.mx/MLM-123456789-filtro-de-aceite-_JM": "MLM123456789",
	}
	for raw, want := range valid {
		got, err := NormalizeItemID(raw)
		if err != nil || got != want {
			t.Errorf("NormalizeItemID(%q) = %q, %v; esperaba %q", raw, got, err, want)
		}
	}

	for _, raw := range []string{"", "abc", "12", "MLA123456789"} {
		if _, err := NormalizeItemID(raw); err == nil {
			t.Errorf("NormalizeItemID(%q) debía fallar", raw)
		}
	}
}

func TestValidatePartNumber(t *testing.T) {
	attrs := func(values ...string) *mercadoLibreInfra.ItemDetail {
		out := []mercadoLibreInfra.ItemAttribute{{ID: "BRAND", ValueName: "Nissan"}}
		for _, v := range values {
			out = append(out, mercadoLibreInfra.ItemAttribute{ID: "PART_NUMBER", ValueName: v})
		}
		return &mercadoLibreInfra.ItemDetail{Attributes: out}
	}

	if err := validatePartNumber("26310-2J000", "SKU-1", attrs("263102j000")); err != nil {
		t.Errorf("debía coincidir ignorando guiones y mayúsculas: %v", err)
	}
	if err := validatePartNumber("26310-2J000", "SKU-1", attrs("AAA111, 26310 2J000")); err != nil {
		t.Errorf("debía coincidir con uno de varios valores: %v", err)
	}

	var mismatch *PartNumberMismatchError
	if err := validatePartNumber("26310-2J000", "SKU-1", attrs("OTRO-123")); !errors.As(err, &mismatch) || len(mismatch.ListingPartNumbers) != 1 {
		t.Errorf("debía reportar discrepancia: %v", err)
	}
	if err := validatePartNumber("26310-2J000", "SKU-1", attrs()); !errors.As(err, &mismatch) || len(mismatch.ListingPartNumbers) != 0 {
		t.Errorf("sin PART_NUMBER debía reportar discrepancia: %v", err)
	}
	if err := validatePartNumber("", "SKU-1", attrs("")); !errors.As(err, &mismatch) {
		t.Errorf("un número de parte vacío nunca debe coincidir: %v", err)
	}

	// Sin PART_NUMBER, el SKU del vendedor se valida contra el número de parte.
	withSKU := attrs()
	withSKU.Attributes = append(withSKU.Attributes, mercadoLibreInfra.ItemAttribute{ID: "SELLER_SKU", ValueName: "26310-2j000"})
	if err := validatePartNumber("26310-2J000", "SKU-1", withSKU); err != nil {
		t.Errorf("SELLER_SKU igual al número de parte debía validar: %v", err)
	}
	legacy := attrs()
	legacy.SellerCustomField = "SKU-1"
	if err := validatePartNumber("26310-2J000", "SKU-1", legacy); err != nil {
		t.Errorf("seller_custom_field igual al SKU debía validar: %v", err)
	}
	legacy.SellerCustomField = "OTRO"
	if err := validatePartNumber("26310-2J000", "SKU-1", legacy); !errors.As(err, &mismatch) {
		t.Errorf("SKU distinto debía reportar discrepancia: %v", err)
	}
}

func TestFormatMeasure(t *testing.T) {
	tests := []struct {
		raw      string
		convert  func(string) (float64, bool)
		decimals int
		want     string
	}{
		{"10 cm", lengthToCM, 2, "10.00"},
		{"12,5 cm", lengthToCM, 2, "12.50"},
		{"100 mm", lengthToCM, 2, "10.00"},
		{"2 in", lengthToCM, 2, "5.08"},
		{"1000 g", weightToKG, 3, "1.000"},
		{"0,5 kg", weightToKG, 3, "0.500"},
		{"750", weightToKG, 3, "0.750"},
		{"10 parsecs", lengthToCM, 2, ""},
		{"sin dato", lengthToCM, 2, ""},
	}
	for _, tt := range tests {
		if got := formatMeasure(tt.raw, tt.convert, tt.decimals); got != tt.want {
			t.Errorf("formatMeasure(%q) = %q, esperaba %q", tt.raw, got, tt.want)
		}
	}
}

func TestBlobName(t *testing.T) {
	tests := []struct {
		sku, partNumber string
		n               int
		want            string
	}{
		{"SKU123", "26310-2J000", 1, "SKU123_26310-2J000_MELI_1.webp"},
		{" SKU 123 ", "AB/CD 9", 12, "SKU-123_AB-CD-9_MELI_12.webp"},
		{"A", "ñandú#1", 3, "A_and-1_MELI_3.webp"},
	}
	for _, tt := range tests {
		if got := blobName(tt.sku, tt.partNumber, tt.n); got != tt.want {
			t.Errorf("blobName(%q, %q, %d) = %q, esperaba %q", tt.sku, tt.partNumber, tt.n, got, tt.want)
		}
	}
}

func TestCompatibilityErrorSummary(t *testing.T) {
	tests := map[string]string{
		"error fetching mercadolibre catalog product MLMU1: Get \"x\": context deadline exceeded": "se agotó el tiempo",
		"mercadolibre items fetch failed: (429): too many requests":                              "MercadoLibre limitó las consultas (429)",
		"mercadolibre item fetch failed: (404): not found":                                       "vehículo no encontrado en el catálogo de MercadoLibre (404)",
		"catalog product MLMU2 is missing brand/model/year attributes":                           "el producto de catálogo no trae marca, modelo o año",
		"error creating vehicle fitment: duplicate":                                              "error creating vehicle fitment",
	}
	for message, want := range tests {
		if got := compatibilityErrorSummary(message); got != want {
			t.Errorf("compatibilityErrorSummary(%q) = %q, esperaba %q", message, got, want)
		}
	}
}
