package meli_report

import (
	"strings"
	"testing"
	"time"
)

func number(v int) *int        { return &v }
func money(v float64) *float64 { return &v }
func at(hour int) *time.Time {
	t := time.Date(2026, 11, 26, hour, 0, 0, 0, time.UTC)
	return &t
}

// listing: último envío a las 12; el stock cambió antes (a las 10).
func listing(status, reason string, stock int) Listing {
	return Listing{
		ConnectionID: 1, ConnectionName: "ML 1", SKU: "A", ExternalID: "MLM1",
		Status: status, PausedReason: reason, Stock: stock,
		LastPushAt: at(12), StockChangedAt: at(10),
	}
}

func live(status string, quantity int, subStatus ...string) Live {
	return Live{Queried: true, Found: true, Status: status, RawStatus: status, SubStatus: subStatus, Quantity: number(quantity)}
}

func TestCompare(t *testing.T) {
	lost := listing("paused", "user", 5)
	lost.PausedForStockByUs = true
	lostWithoutStock := listing("paused", "", 0)
	lostWithoutStock.PausedForStockByUs = true
	full := live("synced", 50)
	full.LogisticType = "fulfillment"
	unknownQuantity := live("synced", 0)
	unknownQuantity.Quantity = nil

	cases := []struct {
		name     string
		local    Listing
		live     Live
		severity Severity // 0 = sin diferencias
		contains string
	}{
		{name: "same", local: listing("synced", "", 3), live: live("synced", 3)},
		{name: "selling without stock", local: listing("synced", "", 0), live: live("synced", 4),
			severity: SeverityUrgent, contains: "SIN stock local"},
		{name: "offering more", local: listing("synced", "", 2), live: live("synced", 4),
			severity: SeverityUrgent, contains: "ofreciendo 4 con stock local 2"},
		{name: "negative stock counts as zero", local: listing("synced", "", -2), live: live("synced", 0)},
		{name: "offering less", local: listing("synced", "", 5), live: live("synced", 3),
			severity: SeverityMedium, contains: "ventas posteriores"},
		{name: "full is not compared", local: listing("synced", "", 0), live: full,
			severity: SeverityInfo, contains: "Full"},
		{name: "unknown quantity is not zero", local: listing("synced", "", 5), live: unknownQuantity,
			severity: SeverityMedium, contains: "no informó la cantidad"},
		{name: "own pause with stock back", local: listing("paused", "zero_stock", 5), live: live("paused", 0, "paused_by_seller"),
			severity: SeverityHigh, contains: "debe reactivarse"},
		{name: "lost zero stock reason", local: lost, live: live("paused", 0, "paused_by_seller"),
			severity: SeverityHigh, contains: "se perdió ese motivo"},
		{name: "lost reason still without stock", local: lostWithoutStock, live: live("paused", 0, "paused_by_seller"),
			severity: SeverityHigh, contains: "no se reactivará sola cuando vuelva"},
		{name: "seller pause with stock", local: listing("paused", "user", 5), live: live("paused", 5, "paused_by_seller"),
			severity: SeverityMedium, contains: "Pausada (por el vendedor)"},
		{name: "out of stock on mercadolibre but we have stock", local: listing("paused", "", 5), live: live("paused", 0, "out_of_stock"),
			severity: SeverityHigh, contains: "MercadoLibre la tiene sin stock"},
		{name: "regular zero stock pause", local: listing("paused", "zero_stock", 0), live: live("paused", 0, "paused_by_seller")},
		{name: "paused with more units than stock", local: listing("paused", "user", 0), live: live("paused", 7, "paused_by_seller"),
			severity: SeverityMedium, contains: "No la reactives"},
		{name: "under review with excess", local: listing("under_review", "", 0), live: live("under_review", 3),
			severity: SeverityHigh, contains: "al activarse"},
		{name: "closed on both", local: listing("closed", "", 0), live: live("closed", 3)},
		{name: "closed only on mercadolibre", local: listing("synced", "", 3), live: live("closed", 0),
			severity: SeverityMedium, contains: "Cerrada en MercadoLibre"},
		{name: "closed only locally and selling without stock", local: listing("closed", "", 0), live: live("synced", 2),
			severity: SeverityUrgent, contains: "SIN stock"},
		{name: "not returned", local: listing("synced", "", 3), live: Live{Queried: true},
			severity: SeverityMedium, contains: "no devolvió"},
		{name: "closed and not returned", local: listing("closed", "", 0), live: Live{Queried: true}},
		{name: "not queried", local: listing("closed", "", 0), live: Live{},
			severity: SeverityMedium, contains: "No se pudo consultar"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := Compare(tc.local, tc.live)
			if tc.severity == 0 {
				if len(findings) != 0 {
					t.Fatalf("unexpected findings: %+v", findings)
				}
				return
			}
			if len(findings) == 0 || findings[0].Severity != tc.severity {
				t.Fatalf("findings = %+v, want severity %s", findings, tc.severity)
			}
			if text := findings[0].Problem + " " + findings[0].Action; !strings.Contains(text, tc.contains) {
				t.Errorf("finding = %q, want %q", text, tc.contains)
			}
		})
	}
}

func TestCompareTellsWhetherARefreshWouldStillSendTheChange(t *testing.T) {
	local := listing("synced", "", 0)

	omitted := Compare(local, live("synced", 4))[0].Problem
	if !strings.Contains(omitted, "no lo reintentará solo") {
		t.Errorf("omitted change: %q", omitted)
	}

	local.StockChangedAt = at(13)
	pending := Compare(local, live("synced", 4))[0].Problem
	if !strings.Contains(pending, "todavía lo enviaría") {
		t.Errorf("pending change: %q", pending)
	}
}

func TestComparePrice(t *testing.T) {
	local := listing("synced", "", 3)
	local.ExpectedPrice = money(150)
	state := live("synced", 3)

	state.Price = money(150.005)
	if findings := Compare(local, state); len(findings) != 0 {
		t.Errorf("rounding must be ignored: %+v", findings)
	}
	state.Price = money(120)
	if findings := Compare(local, state); len(findings) != 1 || findings[0].Severity != SeverityHigh {
		t.Errorf("findings = %+v", findings)
	}

	// El stock de más pesa más que el precio y queda primero.
	state.Quantity = number(9)
	findings := Compare(local, state)
	if len(findings) != 2 || findings[0].Severity != SeverityUrgent {
		t.Errorf("findings = %+v", findings)
	}
}
