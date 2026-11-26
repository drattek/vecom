// Package sales importa las ventas de los marketplaces (solo lectura) y las
// expone a la vista "Ventas". Ver ADR 0010.
package sales

import (
	"context"
	"strings"
	"time"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

// Estados normalizados.
const (
	OrderPending   = "pending"
	OrderConfirmed = "confirmed"
	OrderCancelled = "cancelled"

	PaymentPending           = "pending"
	PaymentPaid              = "paid"
	PaymentPartiallyRefunded = "partially_refunded"
	PaymentRefunded          = "refunded"
	PaymentRejected          = "rejected"

	ShippingPending       = "pending"
	ShippingReadyToShip   = "ready_to_ship"
	ShippingShipped       = "shipped"
	ShippingDelivered     = "delivered"
	ShippingNotDelivered  = "not_delivered"
	ShippingCancelled     = "cancelled"
	ShippingNotApplicable = "not_applicable"

	StatusUnknown = "unknown"
)

// Origen de un cargo del desglose (ver SaleRecord.ChargeSources).
const (
	chargeSourceOrder    = "order"
	chargeSourceShipping = "shipping"

	chargeDiscount = "discount"
	chargeShipping = "shipping"
	chargeTax      = "tax"
	chargeFee      = "fee"

	paidByBuyer       = "buyer"
	paidBySeller      = "seller"
	paidByMarketplace = "marketplace"
)

// Window es el rango que se pide al marketplace. En la carga inicial se filtra
// por fecha de creación; después, por fecha de modificación.
type Window struct {
	From    time.Time
	Initial bool
}

// Candidate es una venta encontrada. Load obtiene su detalle completo; known es
// lo ya guardado (nil si es nueva o no se consultó).
type Candidate struct {
	ExternalID        string
	ExternalUpdatedAt *time.Time
	Load              func(ctx context.Context, known *mysqlInfra.SaleKnownState) (*mysqlInfra.SaleRecord, error)
}

// HandleFunc procesa una venta. Un error significa que hay que detener la
// corrida; los fallos de una sola venta no se devuelven aquí.
type HandleFunc func(ctx context.Context, candidate Candidate) error

// Session es la conexión ya autenticada de una corrida.
type Session interface {
	// Search recorre todas las ventas de la ventana.
	Search(ctx context.Context, window Window, handle HandleFunc) error
	// Lookup vuelve a consultar ventas ya guardadas por su ID externo.
	Lookup(ctx context.Context, externalIDs []string, handle HandleFunc) error
	// Warnings son limitaciones que no impiden importar (p. ej. un modelo sin
	// permiso de lectura).
	Warnings() []string
}

// Reader sabe leer las ventas de un canal (ecom_channels.code).
type Reader interface {
	ChannelCode() string
	Open(ctx context.Context, connectionID int64) (Session, error)
}

// strPtr devuelve nil para un texto vacío: "no se obtuvo".
func strPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func floatPtr(value float64) *float64 { return &value }

// joinNonEmpty une las partes con contenido.
func joinNonEmpty(separator string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, separator)
}
