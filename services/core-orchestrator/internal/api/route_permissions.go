package api

import (
	httpHandler "core-orchestrator/internal/interfaces/http"
)

// routePermissions es la política RBAC del admin-dashboard (ADR 0009): para
// cada "MÉTODO /patrón" lista los permisos "recurso.accion" que dan acceso
// (basta uno). Un usuario admin siempre pasa.
//
// Default-deny: toda ruta protegida que NO aparece aquí es solo para admin.
// Eso cubre a propósito /api/sources, /api/attributes*, /api/channel-attributes*,
// /api/channel-parameters*, /api/migration/*, los endpoints temp-* y debug de
// MercadoLibre, item-lookup y borrado de movimientos de stock.
//
// Las lecturas de catálogos (marcas, categorías, divisas, almacenes, ...)
// admiten también los permisos de las pantallas que las usan en sus
// formularios, para que un rol con "products.view" pueda ver el listado de
// productos sin tener acceso a Configuración.
var routePermissions = buildRoutePermissions()

func buildRoutePermissions() httpHandler.RoutePolicy {
	p := httpHandler.RoutePolicy{}

	// set registra "MÉTODO patrón" → permisos (cualquiera de ellos da acceso).
	set := func(method, pattern string, perms ...string) {
		p[method+" "+pattern] = perms
	}
	// crud registra el CRUD estándar de un recurso. readAlso amplía quién puede
	// leer (GET) el recurso.
	crud := func(base, resource string, readAlso ...string) {
		read := append([]string{resource + ".view"}, readAlso...)
		set("GET", base, read...)
		set("GET", base+"/{id}", read...)
		set("POST", base, resource+".create")
		set("PUT", base+"/{id}", resource+".edit")
		set("DELETE", base+"/{id}", resource+".delete")
	}

	// Sesión: solo requiere autenticación.
	set("POST", "/api/logout", httpHandler.AuthenticatedOnly)
	set("GET", "/api/me", httpHandler.AuthenticatedOnly)

	// ---- Dashboard ---------------------------------------------------------
	for _, path := range []string{
		"/api/dashboard/listings",
		"/api/dashboard/catalog/count",
		"/api/dashboard/catalog/products",
		"/api/dashboard/health/connections",
		"/api/dashboard/health/inventory",
		"/api/dashboard/health/queue",
	} {
		set("GET", path, "dashboard.view")
	}

	// ---- Ventas ------------------------------------------------------------
	// sales.view incluye los datos del cliente (ADR 0010). No da acceso a las
	// rutas de conexiones: el filtro por conexión sale de sync-status.
	set("GET", "/api/sales", "sales.view")
	set("GET", "/api/sales/{id}", "sales.view")
	set("GET", "/api/sales/sync-status", "sales.view")
	set("POST", "/api/sales/sync", "sales.sync")

	// ---- Usuarios, roles y permisos --------------------------------------
	set("GET", "/api/users", "users.view")
	set("GET", "/api/users/{id}", "users.view", "users.edit")
	set("POST", "/api/users", "users.create")
	set("PUT", "/api/users/{id}", "users.edit")

	usersOrRoles := []string{"roles.view", "users.view", "users.create", "users.edit"}
	set("GET", "/api/roles", usersOrRoles...)
	set("GET", "/api/roles/{id}", usersOrRoles...)
	set("GET", "/api/permissions", "roles.view", "roles.create", "roles.edit")
	set("POST", "/api/roles", "roles.create")
	set("PUT", "/api/roles/{id}", "roles.edit")
	set("DELETE", "/api/roles/{id}", "roles.delete")

	// ---- Productos ---------------------------------------------------------
	productReaders := []string{
		"products.view", "products_pending.view", "products_channels.view", "products_images.view",
		"products_import.view", "products_bulk_update.view", "meli_listings.view",
		"channel_listings.view", "reports.view", "dashboard.view",
	}
	set("GET", "/api/products", productReaders...)
	set("GET", "/api/products/{id}", productReaders...)
	set("POST", "/api/products", "products.create")
	set("PUT", "/api/products/{id}", "products.edit")
	set("DELETE", "/api/products/{id}", "products.delete")

	// Detalle de producto: una pestaña = un recurso.
	set("GET", "/api/products/{productId}/details/general", "products.view")
	set("PATCH", "/api/products/{productId}/details/general", "products.edit")
	set("GET", "/api/products/{productId}/details/media", "product_media.view")
	set("GET", "/api/products/{productId}/details/pricing", "product_pricing.view")
	set("GET", "/api/products/{productId}/details/pricing/history", "product_pricing.view")
	set("GET", "/api/products/{productId}/details/inventory", "product_inventory.view")
	set("GET", "/api/products/{productId}/details/inventory/movements", "product_inventory.view")
	set("POST", "/api/products/{productId}/details/inventory/movements", "product_inventory.create")
	set("GET", "/api/products/{productId}/details/part-numbers", "product_part_numbers.view")
	set("GET", "/api/products/{productId}/details/attributes", "product_attributes.view")
	set("GET", "/api/products/{productId}/details/attributes/checklist", "product_attributes.view")
	set("PATCH", "/api/products/{productId}/details/attributes/dimensions", "product_attributes.edit")
	set("PATCH", "/api/products/{productId}/details/attributes/seo", "product_attributes.edit")
	set("GET", "/api/products/{productId}/details/compatibilities", "product_compatibilities.view")
	set("GET", "/api/products/{productId}/details/sync", "product_sync.view")
	set("GET", "/api/products/{productId}/details/sync/{connectionId}/category-suggestions", "product_sync.view", "product_sync.publish")
	set("POST", "/api/products/{productId}/details/sync/{connectionId}/category", "product_sync.publish")
	set("POST", "/api/products/{productId}/details/sync/{connectionId}/import-listing", "product_sync.publish")
	set("GET", "/api/products/{productId}/import-listing-jobs/{jobId}", "product_sync.publish")

	// Atributos del producto (valores propios y de canal).
	set("GET", "/api/products/{productId}/attributes", "product_attributes.view")
	set("PUT", "/api/products/{productId}/attributes", "product_attributes.edit")
	set("DELETE", "/api/product-attributes/{id}", "product_attributes.edit")
	set("POST", "/api/channel-attribute-values", "product_attributes.edit")
	set("POST", "/api/marketplaces/mercadolibre/product-attributes", "product_attributes.edit")
	set("POST", "/api/marketplaces/mercadolibre/product-attributes/provision", "product_attributes.edit")

	// Medios (imágenes, adjuntos, videos).
	mediaReaders := []string{"product_media.view", "products.view", "products_images.view"}
	set("GET", "/api/product-images", mediaReaders...)
	set("GET", "/api/product-images/{id}", mediaReaders...)
	set("GET", "/api/products/{productId}/images", mediaReaders...)
	set("POST", "/api/product-images", "product_media.create")
	set("PUT", "/api/product-images/{id}", "product_media.edit")
	set("DELETE", "/api/product-images/{id}", "product_media.delete")
	set("POST", "/api/product-images/import", "products_images.import")
	set("POST", "/api/products/{productId}/images/bulk", "product_media.create")
	set("PUT", "/api/products/{productId}/images/{id}/replace", "product_media.edit")
	set("PUT", "/api/products/{productId}/images/{id}/cover", "product_media.edit")
	set("POST", "/api/products/{productId}/attachments/bulk", "product_media.create")
	set("PUT", "/api/products/{productId}/attachments/{fileId}/replace", "product_media.edit")
	set("DELETE", "/api/products/{productId}/attachments/{fileId}", "product_media.delete")
	set("POST", "/api/products/{productId}/videos/bulk", "product_media.create")
	set("PUT", "/api/products/{productId}/videos/{id}/replace", "product_media.edit")
	set("GET", "/api/product-videos", mediaReaders...)
	set("GET", "/api/product-videos/{id}", mediaReaders...)
	set("GET", "/api/products/{productId}/videos", mediaReaders...)
	set("POST", "/api/product-videos", "product_media.create")
	set("PUT", "/api/product-videos/{id}", "product_media.edit")
	set("DELETE", "/api/product-videos/{id}", "product_media.delete")

	// Números de parte.
	set("GET", "/api/product-part-numbers", "product_part_numbers.view")
	set("GET", "/api/products/{productId}/part-numbers", "product_part_numbers.view")
	set("POST", "/api/product-part-numbers", "product_part_numbers.create")
	set("PUT", "/api/products/{productId}/part-numbers/{partNumber}", "product_part_numbers.edit")
	set("DELETE", "/api/products/{productId}/part-numbers/{partNumber}", "product_part_numbers.delete")

	// Dimensiones y SEO (parte de "Atributos").
	set("GET", "/api/products/{productId}/dimensions", "product_attributes.view")
	set("POST", "/api/products/{productId}/dimensions", "product_attributes.edit")
	set("PUT", "/api/products/{productId}/dimensions", "product_attributes.edit")
	set("DELETE", "/api/products/{productId}/dimensions", "product_attributes.edit")
	set("POST", "/api/product-dimensions/bulk-import", "product_attributes.edit")
	set("GET", "/api/products/{productId}/seo", "product_attributes.view")
	set("POST", "/api/products/{productId}/seo", "product_attributes.edit")
	set("PUT", "/api/products/{productId}/seo", "product_attributes.edit")
	set("DELETE", "/api/products/{productId}/seo", "product_attributes.edit")

	// Importación y actualización masiva por Excel.
	set("POST", "/api/meli-bulk-import/batches", "products_import.import")
	set("GET", "/api/meli-bulk-import/batches", "products_import.view")
	set("GET", "/api/meli-bulk-import/batches/{batchId}/export", "products_import.export")
	set("GET", "/api/meli-bulk-import/items", "products_import.view")
	set("GET", "/api/meli-bulk-import/summary", "products_import.view")
	set("POST", "/api/meli-bulk-import/items/{itemId}/retry", "products_import.retry")
	set("POST", "/api/meli-bulk-import/retry", "products_import.retry")

	set("POST", "/api/product-bulk-update/batches", "products_bulk_update.import")
	set("GET", "/api/product-bulk-update/batches", "products_bulk_update.view")
	set("GET", "/api/product-bulk-update/batches/{batchId}/export", "products_bulk_update.export")
	set("GET", "/api/product-bulk-update/template", "products_bulk_update.view")
	set("GET", "/api/product-bulk-update/items", "products_bulk_update.view")
	set("GET", "/api/product-bulk-update/summary", "products_bulk_update.view")
	set("POST", "/api/product-bulk-update/items/{itemId}/retry", "products_bulk_update.retry")
	set("POST", "/api/product-bulk-update/retry", "products_bulk_update.retry")

	// ---- Inventario y precios (lecturas también para las pestañas) ---------
	stockReaders := []string{"product_inventory.view"}
	crud("/api/branches", "branches", stockReaders...)
	crud("/api/warehouses", "warehouses", stockReaders...)
	set("GET", "/api/product-stock", "product_inventory.view")
	set("GET", "/api/product-stock/{id}", "product_inventory.view")
	set("POST", "/api/product-stock", "product_inventory.create")
	set("PUT", "/api/product-stock/{id}", "product_inventory.create")
	set("GET", "/api/stock-movements", "product_inventory.view")
	set("GET", "/api/stock-movements/{id}", "product_inventory.view")
	set("GET", "/api/stock-movements/product/{productId}", "product_inventory.view")
	set("POST", "/api/stock-movements", "product_inventory.create")

	priceReaders := []string{"product_pricing.view"}
	crud("/api/currencies", "currencies", priceReaders...)
	crud("/api/exchange-rates", "exchange_rates", priceReaders...)
	crud("/api/price-lists", "price_lists", priceReaders...)
	set("POST", "/api/price-lists/{id}/prices/bulk-import", "price_lists.import")
	crud("/api/pricing-formulas", "pricing_formulas")
	set("GET", "/api/product-prices", "product_pricing.view", "price_lists.view")
	set("GET", "/api/product-prices/{id}", "product_pricing.view", "price_lists.view")
	set("POST", "/api/product-prices", "price_lists.edit")
	set("PUT", "/api/product-prices/{id}", "price_lists.edit")
	set("DELETE", "/api/product-prices/{id}", "price_lists.delete")

	// ---- Almacenamiento ----------------------------------------------------
	crud("/api/storage-disks", "storage_disks", "product_media.view", "products_images.view")
	set("PUT", "/api/storage-disks/{id}/azure-credentials", "storage_disks.edit")
	crud("/api/files", "storage_files", "product_media.view")

	// ---- Canales y conexiones ---------------------------------------------
	channelReaders := []string{
		"products_channels.view", "products.view", "connections.view", "channel_listings.view",
		"meli_listings.view", "categories.view", "product_sync.view",
	}
	crud("/api/channels", "channels", channelReaders...)
	set("GET", "/api/channels/{id}/sync-summary", append([]string{"channels.view"}, channelReaders...)...)

	connReaders := append([]string{"connections.view"}, channelReaders...)
	crud("/api/channel-connections", "connections", connReaders...)
	set("GET", "/api/channel-connections/{id}/config", "connections.manage_credentials")
	set("POST", "/api/channel-connections/{id}/mercadolibre/authorize", "connections.manage_credentials")
	set("GET", "/api/channel-connections/{id}/mercadolibre/app", "connections.manage_credentials")
	set("PUT", "/api/channel-connections/{id}/mercadolibre/app", "connections.manage_credentials")
	set("GET", "/api/connections/{connectionId}/status", connReaders...)
	for _, base := range []string{"/api/connection-credentials", "/api/connection-settings"} {
		set("GET", base, "connections.manage_credentials")
		set("GET", base+"/{id}", "connections.manage_credentials")
		set("POST", base, "connections.manage_credentials")
		set("PUT", base+"/{id}", "connections.manage_credentials")
		set("DELETE", base+"/{id}", "connections.manage_credentials")
	}
	set("GET", "/api/marketplaces/odoo/test-connection", "connections.manage_credentials")

	// ---- Marcas y categorías ----------------------------------------------
	brandReaders := []string{"products.view", "product_sync.view", "products_channels.view"}
	crud("/api/brands", "brands", brandReaders...)
	set("GET", "/api/brands/options", append([]string{"brands.view"}, brandReaders...)...)
	set("POST", "/api/brands/bulk-assign", "brands.bulk_assign")

	categoryReaders := []string{"products.view", "product_sync.view", "product_attributes.view", "products_channels.view"}
	crud("/api/categories", "categories", categoryReaders...)
	set("GET", "/api/categories/{id}/children", append([]string{"categories.view"}, categoryReaders...)...)
	set("GET", "/api/categories/{id}/channel-mappings", append([]string{"categories.view"}, categoryReaders...)...)
	set("GET", "/api/category-import/sources", "categories.import")
	set("GET", "/api/category-import/tree", "categories.import")
	set("POST", "/api/category-import", "categories.import")
	set("POST", "/api/category-import/mapping", "categories.import")

	// ---- Compatibilidades ---------------------------------------------------
	compatReaders := []string{"product_compatibilities.view"}
	crud("/api/equipment-types", "equipment_types", append(compatReaders, "equipment_fitments.view")...)
	crud("/api/vehicle-fitments", "vehicle_fitments", compatReaders...)
	set("POST", "/api/vehicle-fitments/find-or-create", "vehicle_fitments.create", "product_compatibilities.create")
	set("POST", "/api/vehicle-fitments/bulk-import", "vehicle_fitments.import")
	set("POST", "/api/vehicle-fitments/resolve-pending", "vehicle_fitments.edit")
	crud("/api/equipment-fitments", "equipment_fitments", compatReaders...)
	set("POST", "/api/equipment-fitments/find-or-create", "equipment_fitments.create", "product_compatibilities.create")

	set("GET", "/api/products/{productId}/vehicle-compatibilities", "product_compatibilities.view")
	set("POST", "/api/product-vehicle-compatibilities", "product_compatibilities.create")
	set("DELETE", "/api/product-vehicle-compatibilities/{id}", "product_compatibilities.delete")
	set("GET", "/api/products/{productId}/equipment-compatibilities", "product_compatibilities.view")
	set("POST", "/api/product-equipment-compatibilities", "product_compatibilities.create")
	set("DELETE", "/api/product-equipment-compatibilities/{id}", "product_compatibilities.delete")

	// ---- Publicaciones y marketplaces -------------------------------------
	listingReaders := []string{"channel_listings.view", "meli_listings.view"}
	set("GET", "/api/channel-listings", listingReaders...)
	set("GET", "/api/channel-listings/history", listingReaders...)
	set("GET", "/api/channel-listings/queue", listingReaders...)
	set("POST", "/api/channel-listings/publish", "channel_listings.publish")
	set("POST", "/api/channel-listings/publish-zero-stock-eligibles", "channel_listings.publish")
	set("POST", "/api/channel-listings/refresh", "channel_listings.resync")
	set("POST", "/api/channel-listings/resync", "channel_listings.resync")
	set("POST", "/api/channel-listings/resync/enqueue", "channel_listings.resync")
	set("POST", "/api/channel-listings/compatibilities", "channel_listings.resync")
	set("POST", "/api/channel-listings/compatibilities/enqueue", "channel_listings.resync")
	set("POST", "/api/channel-listings/close", "channel_listings.close")
	set("GET", "/api/reports/publications", "reports.view")
	set("GET", "/api/reports/publications/attributes", "reports.view")

	set("GET", "/api/marketplaces/mercadolibre/category-predictor", "product_sync.publish", "channel_listings.publish")
	for _, path := range []string{
		"/api/marketplaces/mercadolibre/products/upload",
		"/api/marketplaces/mercadolibre/products/update-price-stock",
		"/api/marketplaces/mercadolibre/channel-product-map/status",
		"/api/marketplaces/mercadolibre/channel-product-map/sync",
		"/api/marketplaces/mercadolibre/compatibilities/fix-under-review",
		"/api/marketplaces/mercadolibre/compatibilities/copy",
		"/api/marketplaces/mercadolibre/compatibilities/diagnose",
		"/api/marketplaces/mercadolibre/listings/sync-flagged",
	} {
		set("POST", path, "mercadolibre.sync")
	}

	return p
}
