package sync

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMercadoLibreFamilyName(t *testing.T) {
	nissan := nissanBrandID
	other := int64(999)

	cases := []struct {
		name    string
		brandID *int64
		title   string
		brand   string
		pn      string
		want    string
	}{
		{"agrega el número de parte", &other, "Filtro De Cabina", "Acme", "SAR1234", "Filtro De Cabina Acme SAR1234"},
		{"ya está en el título", &other, "Filtro SAR1234 Cabina", "Acme", "SAR1234", "Filtro SAR1234 Cabina Acme"},
		{"ya está, sin distinguir mayúsculas", &other, "Filtro sar1234", "Acme", "SAR1234", "Filtro sar1234 Acme"},
		{"no confunde un número más largo", &other, "Filtro 1234 Cabina", "Acme", "123", "Filtro 1234 Cabina Acme 123"},
		{"sin número de parte", &other, "Filtro De Cabina", "Acme", " ", "Filtro De Cabina Acme"},
		{"Nissan conserva Original", &nissan, "Filtro", "Nissan", "SAR1234", "Filtro Original Nissan SAR1234"},
		{"no excede 60 caracteres", &other, strings.Repeat("a", 45), "Acme", "SAR1234567", strings.Repeat("a", 45) + " Acme"},
	}
	for _, c := range cases {
		got := mercadoLibreFamilyName(c.brandID, c.title, c.brand, c.pn)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
		if utf8.RuneCountInString(got) > 60 && utf8.RuneCountInString(c.title) <= 40 {
			t.Errorf("%s: %q excede 60 caracteres", c.name, got)
		}
	}
}
