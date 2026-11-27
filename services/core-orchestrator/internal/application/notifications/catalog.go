// Package notifications gestiona los avisos por Telegram (ADR 0012):
// configuración del bot, vinculación de usuarios y preferencias.
package notifications

import "core-orchestrator/internal/domain"

// Tipos de aviso (ecom_notification_preferences.notification_type).
const (
	TypeErpSyncFailure      = "erp_sync_failure"
	TypeConnectionDown      = "connection_down"
	TypePublicationErrors   = "publication_errors"
	TypeStockPauseSummary   = "stock_pause_summary"
	TypeMeliListingClosed   = "meli_listing_closed"
	TypeBulkProcessFinished = "bulk_process_finished"
)

// Cuándo se envía cada aviso.
const (
	DeliveryInstant      = "instant"
	DeliveryAfterBatch   = "after_batch"
	DeliveryDailySummary = "daily_summary"
	DeliveryAfterCheck   = "after_check"
)

// Definition describe un tipo de aviso y quién puede recibirlo.
type Definition struct {
	Type        string
	Group       string
	Label       string
	Description string
	Delivery    string
	// Active: el aviso ya se envía. Los demás llegan en fases posteriores.
	Active bool
	// Permissions: basta uno (ADR 0009). admin siempre puede.
	Permissions []string
}

var catalog = []Definition{
	{
		Type: TypeErpSyncFailure, Group: "Sistema", Delivery: DeliveryInstant, Active: true,
		Label:       "Falla en la sincronización con el ERP",
		Description: "Fallos, registros que no se pudieron sincronizar y falta de sincronización.",
		Permissions: []string{"dashboard.view"},
	},
	{
		Type: TypeConnectionDown, Group: "Sistema", Delivery: DeliveryInstant, Active: true,
		Label:       "Conexión de marketplace caída o token vencido",
		Description: "Cuando una conexión deja de funcionar y cuando se recupera.",
		Permissions: []string{"connections.view"},
	},
	{
		Type: TypePublicationErrors, Group: "Publicaciones", Delivery: DeliveryAfterBatch, Active: true,
		Label:       "Errores al publicar o resincronizar",
		Description: "Productos con error al terminar cada tanda.",
		Permissions: []string{"channel_listings.view", "reports.view"},
	},
	{
		Type: TypeStockPauseSummary, Group: "Publicaciones", Delivery: DeliveryDailySummary, Active: true,
		Label:       "Publicaciones pausadas por falta de stock",
		Description: "Un renglón por producto con todas sus publicaciones pausadas.",
		Permissions: []string{"channel_listings.view", "products_channels.view"},
	},
	{
		Type: TypeMeliListingClosed, Group: "Publicaciones", Delivery: DeliveryAfterCheck, Active: true,
		Label:       "Publicaciones cerradas por MercadoLibre",
		Description: "Cierres definitivos con su motivo, revisados a las 0:00 y a las 12:00.",
		Permissions: []string{"channel_listings.view", "meli_listings.view"},
	},
	{
		Type: TypeBulkProcessFinished, Group: "Procesos masivos", Delivery: DeliveryInstant, Active: true,
		Label:       "Terminó mi importación o actualización masiva",
		Description: "Solo de los procesos que tú inicies.",
		Permissions: []string{"products_import.view", "products_bulk_update.view"},
	},
}

func definitionOf(notificationType string) (Definition, bool) {
	for _, definition := range catalog {
		if definition.Type == notificationType {
			return definition, true
		}
	}
	return Definition{}, false
}

// AllowedDefinitions son los avisos que el usuario puede configurar y recibir.
func AllowedDefinitions(user *domain.AuthUser) []Definition {
	allowed := make([]Definition, 0, len(catalog))
	for _, definition := range catalog {
		if user.HasAnyPermission(definition.Permissions...) {
			allowed = append(allowed, definition)
		}
	}
	return allowed
}
