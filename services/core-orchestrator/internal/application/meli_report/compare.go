// Package meli_report compara las publicaciones locales con lo que MercadoLibre
// muestra ahora. Es de solo lectura: no cambia ni la base ni las publicaciones.
package meli_report

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Severity ordena el reporte: menor es más urgente.
type Severity int

const (
	SeverityUrgent Severity = iota + 1
	SeverityHigh
	SeverityMedium
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityUrgent:
		return "1-URGENTE"
	case SeverityHigh:
		return "2-ALTA"
	case SeverityMedium:
		return "3-MEDIA"
	default:
		return "4-INFO"
	}
}

const (
	statusSynced      = "synced"
	statusPaused      = "paused"
	statusUnderReview = "under_review"
	statusClosed      = "closed"

	reasonZeroStock = "zero_stock"

	subStatusOutOfStock     = "out_of_stock"
	subStatusPausedBySeller = "paused_by_seller"
	logisticFulfillment     = "fulfillment"

	priceTolerance = 0.01
)

// Listing es una publicación según la base local.
type Listing struct {
	ConnectionID   int64
	ConnectionName string
	ProductID      int64
	BrandID        *int64
	SKU            string
	ProductName    string
	ExternalID     string
	Status         string
	PausedReason   string
	LastPushAt     *time.Time
	Stock          int
	StockChangedAt *time.Time
	// ExpectedPrice es nil si no se pudo calcular; PriceNote dice por qué.
	ExpectedPrice  *float64
	PriceChangedAt *time.Time
	PriceNote      string
	// PausedForStockByUs: según la bitácora, la última pausa o reactivación fue
	// una pausa de este sistema por falta de stock.
	PausedForStockByUs bool
}

// Live es la publicación según MercadoLibre. Status ya viene en el vocabulario
// local.
type Live struct {
	// Queried es false si no se pudo consultar.
	Queried bool
	// Found es false si MercadoLibre no la devolvió.
	Found        bool
	Status       string
	RawStatus    string
	SubStatus    []string
	Quantity     *int
	Price        *float64
	LogisticType string
}

// Finding es una diferencia encontrada.
type Finding struct {
	Severity Severity
	Problem  string
	Action   string
}

func (l Live) has(subStatus string) bool {
	for _, value := range l.SubStatus {
		if strings.EqualFold(value, subStatus) {
			return true
		}
	}
	return false
}

// pendingPush indica si el precio o el stock cambiaron después del último envío:
// entonces el siguiente refresco todavía los mandaría.
func (l Listing) pendingPush() bool {
	if l.LastPushAt == nil {
		return true
	}
	after := func(t *time.Time) bool { return t != nil && t.After(*l.LastPushAt) }
	return after(l.StockChangedAt) || after(l.PriceChangedAt)
}

func retryNote(listing Listing) string {
	if listing.pendingPush() {
		return "Hay un cambio posterior al último envío: un refresco todavía lo enviaría."
	}
	return "Para el sistema ya está enviado: el refresco no lo reintentará solo."
}

// Compare devuelve las diferencias de una publicación, de la más a la menos
// grave. Sin diferencias devuelve nil.
func Compare(listing Listing, live Live) []Finding {
	if !live.Queried {
		return []Finding{{SeverityMedium, "No se pudo consultar en MercadoLibre.", "Repite el reporte."}}
	}
	if !live.Found {
		if listing.Status == statusClosed {
			return nil
		}
		return []Finding{{SeverityMedium,
			fmt.Sprintf("MercadoLibre no devolvió la publicación (local: %s).", listing.Status),
			"Revísala en MercadoLibre: puede estar eliminada o ser de otra cuenta."}}
	}

	if live.Status == statusClosed {
		if listing.Status == statusClosed {
			return nil
		}
		return []Finding{{SeverityMedium,
			fmt.Sprintf("Cerrada en MercadoLibre, pero localmente está en %s.", listing.Status),
			"Sincroniza el estado."}}
	}

	var findings []Finding
	add := func(severity Severity, problem, action string) {
		findings = append(findings, Finding{severity, problem, action})
	}
	full := strings.EqualFold(live.LogisticType, logisticFulfillment)

	if live.Status != listing.Status {
		severity := SeverityMedium
		if listing.Status == statusClosed {
			severity = SeverityHigh
		}
		add(severity, fmt.Sprintf("Estado distinto: local %s, MercadoLibre %s.", listing.Status, live.Status),
			"Sincroniza el estado.")
	}

	switch {
	case full:
		add(SeverityInfo, "Publicación Full: MercadoLibre administra su stock; no se compara.", "")
	case live.Quantity == nil:
		add(SeverityMedium, "MercadoLibre no informó la cantidad disponible.", "Revísala en MercadoLibre.")
	default:
		compareStock(listing, live, *live.Quantity, add)
	}

	if live.Status == statusPaused {
		comparePause(listing, live, full, add)
	}

	// A una publicación pausada sin stock no se le envía precio.
	if live.Status == statusSynced && live.Price != nil && listing.ExpectedPrice != nil && *listing.ExpectedPrice > 0 &&
		math.Abs(*live.Price-*listing.ExpectedPrice) > priceTolerance {
		add(SeverityHigh,
			fmt.Sprintf("Precio distinto: MercadoLibre %.2f, calculado %.2f. %s", *live.Price, *listing.ExpectedPrice, retryNote(listing)),
			"Revisa el precio (puede tener una promoción) y resincroniza si corresponde.")
	}

	sort.SliceStable(findings, func(i, j int) bool { return findings[i].Severity < findings[j].Severity })
	return findings
}

func compareStock(listing Listing, live Live, quantity int, add func(Severity, string, string)) {
	local := max(listing.Stock, 0)
	switch {
	case quantity > local && live.Status == statusSynced:
		problem := fmt.Sprintf("Activa ofreciendo %d con stock local %d.", quantity, local)
		if local == 0 {
			problem = fmt.Sprintf("Activa ofreciendo %d SIN stock local.", quantity)
		}
		add(SeverityUrgent, problem+" "+retryNote(listing), "Confirma el stock y resincroniza ya.")
	case quantity > local && live.Status == statusUnderReview:
		add(SeverityHigh,
			fmt.Sprintf("En revisión con %d unidades y stock local %d: al activarse ofrecería de más.", quantity, local),
			"Resincroniza al corregirla.")
	case quantity > local:
		add(SeverityMedium,
			fmt.Sprintf("Pausada con %d unidades en MercadoLibre y stock local %d.", quantity, local),
			"No la reactives sin enviar antes el stock.")
	case quantity < local && live.Status == statusSynced:
		add(SeverityMedium,
			fmt.Sprintf("Activa ofreciendo %d con stock local %d. Puede deberse a ventas posteriores a la última sincronización del ERP. %s",
				quantity, local, retryNote(listing)),
			"Confirma las ventas recientes antes de resincronizar.")
	}
}

func comparePause(listing Listing, live Live, full bool, add func(Severity, string, string)) {
	ownPause := listing.PausedReason == reasonZeroStock
	lostReason := !ownPause && listing.PausedForStockByUs && !live.has(subStatusOutOfStock)

	if listing.Stock <= 0 || full {
		if lostReason {
			add(SeverityHigh,
				"La pausó este sistema por falta de stock y se perdió ese motivo: no se reactivará sola cuando vuelva el stock.",
				"Al recuperar stock, resincroniza y actívala en MercadoLibre.")
		}
		return
	}

	switch {
	case ownPause:
		add(SeverityHigh,
			fmt.Sprintf("Pausada por falta de stock, pero ya hay %d unidades.", listing.Stock),
			"Resincroniza: debe reactivarse.")
	case lostReason:
		add(SeverityHigh,
			fmt.Sprintf("Hay %d unidades y sigue pausada. La pausó este sistema por stock y se perdió ese motivo: no se reactivará sola.", listing.Stock),
			"Confirma que nadie la haya pausado a propósito, envía el stock y actívala.")
	case live.has(subStatusOutOfStock):
		add(SeverityHigh,
			fmt.Sprintf("MercadoLibre la tiene sin stock, pero hay %d unidades. %s", listing.Stock, retryNote(listing)),
			"Resincroniza: al recibir stock se activa sola.")
	default:
		detail := "sin detalle"
		if len(live.SubStatus) > 0 {
			detail = strings.Join(live.SubStatus, ", ")
		}
		if live.has(subStatusPausedBySeller) {
			detail = "por el vendedor"
		}
		add(SeverityMedium,
			fmt.Sprintf("Pausada (%s) con %d unidades en stock.", detail, listing.Stock),
			"Confirma si la pausa es intencional antes de activarla.")
	}
}
