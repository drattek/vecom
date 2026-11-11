package sync

import (
	"testing"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

func TestNormalizeOdooTitle(t *testing.T) {
	cases := []struct{ title, pn, want string }{
		{"FILTRO DE CABINA NISSAN", "sar1234", "Filtro De Cabina Nissan SAR1234"},
		{"Filtro De Cabina Nissan SAR1234", "SAR1234", "Filtro De Cabina Nissan SAR1234"},
		{"FILTRO SAR1234 NISSAN", "SAR1234", "Filtro SAR1234 Nissan"},
		{"filtro sar1234 nissan", "sar1234", "Filtro SAR1234 Nissan"},
		{"Filtro 1234 Nissan", "123", "Filtro 1234 Nissan 123"},
		{"Filtro Nissan", "", "Filtro Nissan"},
		{"Filtro Nissan SAR1234 SAR1234", "SAR1234", "Filtro Nissan SAR1234"},
		{"FILTRO SAR1234 NISSAN sar1234", "SAR1234", "Filtro SAR1234 Nissan"},
		{"Filtro Nissan (SAR-12)", "SAR-12", "Filtro Nissan (SAR-12)"},
	}
	for _, c := range cases {
		if got := normalizeOdooTitle(c.title, c.pn); got != c.want {
			t.Errorf("normalizeOdooTitle(%q,%q) = %q, want %q", c.title, c.pn, got, c.want)
		}
		// idempotente
		if got := normalizeOdooTitle(c.want, c.pn); got != c.want {
			t.Errorf("no idempotente para %q: %q", c.want, got)
		}
	}
}

func TestResolveOdooRefreshTitle(t *testing.T) {
	stale := "Titulo Viejo SAR1234"
	fitment := int64(7)
	product := &mysqlInfra.ProductDTO{Name: "TITULO NUEVO", PartNumber: "SAR1234"}

	general := &mysqlInfra.ChannelProductMapDTO{ListingTitle: &stale}
	if got, want := resolveOdooRefreshTitle(general, product), "Titulo Nuevo SAR1234"; got != want {
		t.Errorf("general = %q, want %q", got, want)
	}

	perFitment := &mysqlInfra.ChannelProductMapDTO{ListingTitle: &stale, VehicleFitmentID: &fitment}
	if got, want := resolveOdooRefreshTitle(perFitment, product), "Titulo Viejo SAR1234"; got != want {
		t.Errorf("fitment = %q, want %q", got, want)
	}
}
