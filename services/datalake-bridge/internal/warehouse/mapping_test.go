package warehouse

import "testing"

func TestOdooCode(t *testing.T) {
	cases := []struct {
		company, site, want string
		wantErr             bool
	}{
		{"msb", "LEN", "WH", false}, // León de msb es el almacén preexistente
		{"MSB", "len", "WH", false}, // insensible a mayúsculas/espacios
		{"msb", "AGS", "AGS", false},
		{"msb", "QRO", "QRO", false},
		{"vrs", "LEN", "VLEN", false}, // León de vrs es otro almacén
		{"vrs", "AGS", "", true},      // vrs solo tiene LEN
		{"xxx", "LEN", "", true},      // empresa sin mapeo
		{"msb", "ZZZ", "", true},      // sucursal sin almacén
	}
	for _, c := range cases {
		got, err := OdooCode(c.company, c.site)
		if c.wantErr {
			if err == nil {
				t.Errorf("OdooCode(%q,%q) esperaba error", c.company, c.site)
			}
			continue
		}
		if err != nil {
			t.Errorf("OdooCode(%q,%q) error inesperado: %v", c.company, c.site, err)
			continue
		}
		if got != c.want {
			t.Errorf("OdooCode(%q,%q) = %q, quiero %q", c.company, c.site, got, c.want)
		}
	}
}
