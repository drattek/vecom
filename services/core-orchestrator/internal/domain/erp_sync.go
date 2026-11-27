package domain

// Flujos del ERP (ecom_erp_sync_state.feed_code).
const (
	ErpFeedProducts    = "products"
	ErpFeedStock       = "stock"
	ErpFeedNissanStock = "nissan_stock"
)

// maxErpFailureSamples limita el detalle que se guarda de una corrida.
const maxErpFailureSamples = 5000

// ErpRecordFailure es un registro que no se pudo sincronizar.
type ErpRecordFailure struct {
	Code  string
	Error string
}

// ErpSyncResult es el saldo de una sincronización que llegó a terminar.
type ErpSyncResult struct {
	Feed   string
	Total  int
	Failed int
	// Failures es el detalle disponible; puede ser menor que Failed.
	Failures []ErpRecordFailure
}

// AddFailure guarda el detalle de un registro fallido, hasta un máximo.
func (r *ErpSyncResult) AddFailure(code string, err error) {
	if len(r.Failures) < maxErpFailureSamples {
		r.Failures = append(r.Failures, ErpRecordFailure{Code: code, Error: err.Error()})
	}
}
