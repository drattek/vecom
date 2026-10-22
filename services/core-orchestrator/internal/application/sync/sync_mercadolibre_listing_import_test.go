package sync

import "testing"

func TestComposeBrandedTitle(t *testing.T) {
	nissan := nissanBrandID
	other := int64(36)

	tests := []struct {
		name       string
		brandID    *int64
		brandName  string
		title      string
		want       string
		wantSkipOK bool // true when a skip reason is expected
	}{
		{name: "marca genérica", brandID: &other, brandName: "Omniparts", title: "Filtro de aceite Sentra", want: "Filtro de aceite Sentra Omniparts"},
		{name: "nissan agrega Original", brandID: &nissan, brandName: "Nissan", title: "Filtro de aceite Sentra", want: "Filtro de aceite Sentra Original Nissan"},
		{name: "nissan con Original ya presente", brandID: &nissan, brandName: "Nissan", title: "Filtro Original Sentra", want: "Filtro Original Sentra Nissan"},
		{name: "ya contiene la marca", brandID: &nissan, brandName: "Nissan", title: "Filtro NISSAN Sentra", wantSkipOK: true},
		{name: "ya contiene la marca con acentos y signos", brandID: &other, brandName: "Ómniparts", title: "Filtro, omniparts. Sentra", wantSkipOK: true},
		{name: "marca como parte de otra palabra no cuenta", brandID: &other, brandName: "Omni", title: "Filtro Omniparts Sentra", want: "Filtro Omniparts Sentra Omni"},
		{name: "sin marca", brandID: nil, brandName: "", title: "Filtro", wantSkipOK: true},
		{name: "excede 60 caracteres", brandID: &other, brandName: "Omniparts", title: "Filtro de aceite para motor Nissan Sentra Versa March Tiida", wantSkipOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := ComposeBrandedTitle(tt.brandID, tt.brandName, tt.title)
			if tt.wantSkipOK {
				if got != "" || reason == "" {
					t.Fatalf("esperaba omitir con motivo, obtuvo title=%q reason=%q", got, reason)
				}
				return
			}
			if got != tt.want || reason != "" {
				t.Fatalf("title=%q reason=%q, esperaba %q", got, reason, tt.want)
			}
		})
	}
}
