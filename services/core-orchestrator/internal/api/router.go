package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	httpHandler "core-orchestrator/internal/interfaces/http"
)

func NewRouter(
	productHandler *httpHandler.ProductHandler,
	storageDiskHandler *httpHandler.StorageDiskHandler,
	fileHandler *httpHandler.FileHandler,
	channelHandler *httpHandler.ChannelHandler,
	channelConnectionHandler *httpHandler.ChannelConnectionHandler,
	authHandler *httpHandler.AuthHandler,
	authMiddleware *httpHandler.JWTMiddleware,
	brandHandler *httpHandler.BrandHandler,
	categoryHandler *httpHandler.CategoryHandler,
	currencyHandler *httpHandler.CurrencyHandler,
	branchHandler *httpHandler.BranchHandler,
	warehouseHandler *httpHandler.WarehouseHandler,
	productStockHandler *httpHandler.ProductStockHandler,
	priceListHandler *httpHandler.PriceListHandler,
	productPricesHandler *httpHandler.ProductPricesHandler,
	exchangeRatesHandler *httpHandler.ExchangeRatesHandler,
	stockMovementsHandler *httpHandler.StockMovementsHandler,
	productImagesHandler *httpHandler.ProductImagesHandler,
	productImageImportHandler *httpHandler.ProductImageImportHandler,
	productVideosHandler *httpHandler.ProductVideosHandler,
	productPartNumbersHandler *httpHandler.ProductPartNumbersHandler,
	productDimensionsHandler *httpHandler.ProductDimensionsHandler,
	productSEOHandler *httpHandler.ProductSEOHandler,
	connectionCredentialsHandler *httpHandler.ConnectionCredentialsHandler,
	connectionSettingsHandler *httpHandler.ConnectionSettingsHandler,
	connectionStatusHandler *httpHandler.ConnectionStatusHandler,
	channelParametersHandler *httpHandler.ChannelParametersHandler,
	mercadoLibreHandler *httpHandler.MercadoLibreHandler,
	meliNotificationHandler *httpHandler.MeliNotificationHandler,
	odooHandler *httpHandler.OdooHandler,
	sourceHandler *httpHandler.SourceHandler,
	migrationHandler *httpHandler.MigrationHandler,
	equipmentTypeHandler *httpHandler.EquipmentTypeHandler,
	vehicleFitmentHandler *httpHandler.VehicleFitmentHandler,
	equipmentFitmentHandler *httpHandler.EquipmentFitmentHandler,
	productVehicleCompatibilityHandler *httpHandler.ProductVehicleCompatibilityHandler,
	productEquipmentCompatibilityHandler *httpHandler.ProductEquipmentCompatibilityHandler,
	channelSyncQueueHandler *httpHandler.ChannelSyncQueueHandler,
	channelListingsHandler *httpHandler.ChannelListingsHandler,
	mercadoLibreItemLookupHandler *httpHandler.MercadoLibreItemLookupHandler,
	mercadoLibrePauseUnmappedListingsHandler *httpHandler.MercadoLibrePauseUnmappedListingsHandler,
	attributeHandler *httpHandler.AttributeHandler,
	attributeOptionHandler *httpHandler.AttributeOptionHandler,
	productAttributeHandler *httpHandler.ProductAttributeHandler,
	channelAttributeHandler *httpHandler.ChannelAttributeHandler,
	channelAttributeMapHandler *httpHandler.ChannelAttributeMapHandler,
	channelAttributeValueHandler *httpHandler.ChannelAttributeValueHandler,
	mercadoLibreCompatibilitiesHandler *httpHandler.MercadoLibreCompatibilitiesHandler,
) http.Handler {

	r := chi.NewRouter()

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Product routes
	r.Post("/api/login", authHandler.Login)

	// MercadoLibre OAuth: the callback path (no /api prefix) is what's
	// registered as the app's redirect_uri in MercadoLibre's developer
	// console, and MercadoLibre hits it directly via browser redirect with no
	// way to attach our own auth — so it's public. GetAuthorizationURL is
	// public too, since it's meant to be opened directly in a browser
	// (window.location) to kick off that same redirect chain.
	r.Get("/meli_callback", mercadoLibreHandler.OAuthCallback)
	r.Get("/api/marketplaces/mercadolibre/oauth/authorize", mercadoLibreHandler.GetAuthorizationURL)

	// MercadoLibre notifications: registered as this app's notification
	// callback URL in MercadoLibre's developer console. MercadoLibre POSTs
	// here directly with no way to attach our own JWT, so it's public, same
	// reasoning as /meli_callback above.
	r.Post("/meli_notifications", meliNotificationHandler.Receive)

	r.Group(func(protected chi.Router) {
		protected.Use(authMiddleware.RequireAuth)
		protected.Get("/api/products", productHandler.GetProducts)
		protected.Get("/api/products/{id}", productHandler.GetProductByID)
		protected.Post("/api/products", productHandler.CreateProduct)
		protected.Put("/api/products/{id}", productHandler.UpdateProduct)
		protected.Delete("/api/products/{id}", productHandler.DeleteProduct)
		protected.Get("/api/storage-disks", storageDiskHandler.GetStorageDisks)
		protected.Get("/api/storage-disks/{id}", storageDiskHandler.GetStorageDiskByID)
		protected.Post("/api/storage-disks", storageDiskHandler.CreateStorageDisk)
		protected.Put("/api/storage-disks/{id}", storageDiskHandler.UpdateStorageDisk)
		protected.Delete("/api/storage-disks/{id}", storageDiskHandler.DeleteStorageDisk)
		protected.Get("/api/files", fileHandler.GetFiles)
		protected.Get("/api/files/{id}", fileHandler.GetFileByID)
		protected.Post("/api/files", fileHandler.CreateFile)
		protected.Put("/api/files/{id}", fileHandler.UpdateFile)
		protected.Delete("/api/files/{id}", fileHandler.DeleteFile)
		protected.Get("/api/channels", channelHandler.GetChannels)
		protected.Get("/api/channels/{id}", channelHandler.GetChannelByID)
		protected.Post("/api/channels", channelHandler.CreateChannel)
		protected.Put("/api/channels/{id}", channelHandler.UpdateChannel)
		protected.Delete("/api/channels/{id}", channelHandler.DeleteChannel)
		protected.Get("/api/channel-connections", channelConnectionHandler.GetChannelConnections)
		protected.Get("/api/channel-connections/{id}", channelConnectionHandler.GetChannelConnectionByID)
		protected.Post("/api/channel-connections", channelConnectionHandler.CreateChannelConnection)
		protected.Put("/api/channel-connections/{id}", channelConnectionHandler.UpdateChannelConnection)
		protected.Delete("/api/channel-connections/{id}", channelConnectionHandler.DeleteChannelConnection)

		// Brands endpoints
		protected.Get("/api/brands", brandHandler.GetBrands)
		protected.Get("/api/brands/{id}", brandHandler.GetBrandByID)
		protected.Post("/api/brands", brandHandler.CreateBrand)
		protected.Put("/api/brands/{id}", brandHandler.UpdateBrand)
		protected.Delete("/api/brands/{id}", brandHandler.DeleteBrand)

		// Categories endpoints
		protected.Get("/api/categories", categoryHandler.GetCategories)
		protected.Get("/api/categories/{id}", categoryHandler.GetCategoryByID)
		protected.Get("/api/categories/{id}/children", categoryHandler.GetCategoryChildren)
		protected.Post("/api/categories", categoryHandler.CreateCategory)
		protected.Put("/api/categories/{id}", categoryHandler.UpdateCategory)
		protected.Delete("/api/categories/{id}", categoryHandler.DeleteCategory)

		// Currencies endpoints
		protected.Get("/api/currencies", currencyHandler.GetCurrencies)
		protected.Get("/api/currencies/{id}", currencyHandler.GetCurrencyByID)
		protected.Post("/api/currencies", currencyHandler.CreateCurrency)
		protected.Put("/api/currencies/{id}", currencyHandler.UpdateCurrency)
		protected.Delete("/api/currencies/{id}", currencyHandler.DeleteCurrency)

		// Branches endpoints
		protected.Get("/api/branches", branchHandler.GetBranches)
		protected.Get("/api/branches/{id}", branchHandler.GetBranchByID)
		protected.Post("/api/branches", branchHandler.CreateBranch)
		protected.Put("/api/branches/{id}", branchHandler.UpdateBranch)
		protected.Delete("/api/branches/{id}", branchHandler.DeleteBranch)

		// Warehouses endpoints
		protected.Get("/api/warehouses", warehouseHandler.GetWarehouses)
		protected.Get("/api/warehouses/{id}", warehouseHandler.GetWarehouseByID)
		protected.Post("/api/warehouses", warehouseHandler.CreateWarehouse)
		protected.Put("/api/warehouses/{id}", warehouseHandler.UpdateWarehouse)
		protected.Delete("/api/warehouses/{id}", warehouseHandler.DeleteWarehouse)

		// Product Stock endpoints
		protected.Get("/api/product-stock", productStockHandler.GetProductStock)
		protected.Get("/api/product-stock/{id}", productStockHandler.GetProductStockByID)
		protected.Post("/api/product-stock", productStockHandler.CreateProductStock)
		protected.Put("/api/product-stock/{id}", productStockHandler.UpdateProductStock)

		// Price Lists endpoints
		protected.Get("/api/price-lists", priceListHandler.GetPriceLists)
		protected.Get("/api/price-lists/{id}", priceListHandler.GetPriceListByID)
		protected.Post("/api/price-lists", priceListHandler.CreatePriceList)
		protected.Put("/api/price-lists/{id}", priceListHandler.UpdatePriceList)
		protected.Delete("/api/price-lists/{id}", priceListHandler.DeletePriceList)

		// Product Prices endpoints
		protected.Get("/api/product-prices", productPricesHandler.GetProductPrices)
		protected.Get("/api/product-prices/{id}", productPricesHandler.GetProductPriceByID)
		protected.Post("/api/product-prices", productPricesHandler.CreateProductPrice)
		protected.Put("/api/product-prices/{id}", productPricesHandler.UpdateProductPrice)
		protected.Delete("/api/product-prices/{id}", productPricesHandler.DeleteProductPrice)

		// Exchange Rates endpoints
		protected.Get("/api/exchange-rates", exchangeRatesHandler.GetExchangeRates)
		protected.Get("/api/exchange-rates/{id}", exchangeRatesHandler.GetExchangeRateByID)
		protected.Post("/api/exchange-rates", exchangeRatesHandler.CreateExchangeRate)
		protected.Put("/api/exchange-rates/{id}", exchangeRatesHandler.UpdateExchangeRate)
		protected.Delete("/api/exchange-rates/{id}", exchangeRatesHandler.DeleteExchangeRate)

		// Stock Movements endpoints
		protected.Get("/api/stock-movements", stockMovementsHandler.GetStockMovements)
		protected.Get("/api/stock-movements/{id}", stockMovementsHandler.GetMovementByID)
		protected.Get("/api/stock-movements/product/{productId}", stockMovementsHandler.GetMovementsByProduct)
		protected.Post("/api/stock-movements", stockMovementsHandler.CreateMovement)
		protected.Delete("/api/stock-movements/{id}", stockMovementsHandler.DeleteMovement)

		// Product Images endpoints
		protected.Get("/api/product-images", productImagesHandler.GetProductImages)
		protected.Get("/api/product-images/{id}", productImagesHandler.GetImageByID)
		protected.Get("/api/products/{productId}/images", productImagesHandler.GetImagesByProduct)
		protected.Post("/api/product-images", productImagesHandler.CreateImage)
		protected.Put("/api/product-images/{id}", productImagesHandler.UpdateImage)
		protected.Delete("/api/product-images/{id}", productImagesHandler.DeleteImage)
		protected.Post("/api/product-images/import", productImageImportHandler.Import)

		// Product Videos endpoints
		protected.Get("/api/product-videos", productVideosHandler.GetProductVideos)
		protected.Get("/api/product-videos/{id}", productVideosHandler.GetVideoByID)
		protected.Get("/api/products/{productId}/videos", productVideosHandler.GetVideosByProduct)
		protected.Post("/api/product-videos", productVideosHandler.CreateVideo)
		protected.Put("/api/product-videos/{id}", productVideosHandler.UpdateVideo)
		protected.Delete("/api/product-videos/{id}", productVideosHandler.DeleteVideo)

		// Product Part Numbers endpoints
		protected.Get("/api/product-part-numbers", productPartNumbersHandler.GetPartNumbers)
		protected.Get("/api/products/{productId}/part-numbers", productPartNumbersHandler.GetPartNumbersByProduct)
		protected.Post("/api/product-part-numbers", productPartNumbersHandler.CreatePartNumber)
		protected.Put("/api/products/{productId}/part-numbers/{partNumber}", productPartNumbersHandler.UpdatePartNumber)
		protected.Delete("/api/products/{productId}/part-numbers/{partNumber}", productPartNumbersHandler.DeletePartNumber)

		// Product Dimensions endpoints
		protected.Get("/api/products/{productId}/dimensions", productDimensionsHandler.GetDimensionsByProduct)
		protected.Post("/api/products/{productId}/dimensions", productDimensionsHandler.CreateDimensions)
		protected.Put("/api/products/{productId}/dimensions", productDimensionsHandler.UpdateDimensions)
		protected.Delete("/api/products/{productId}/dimensions", productDimensionsHandler.DeleteDimensions)
		protected.Post("/api/product-dimensions/bulk-import", productDimensionsHandler.BulkUpsertDimensions)

		// Product SEO endpoints
		protected.Get("/api/products/{productId}/seo", productSEOHandler.GetSEOByProduct)
		protected.Post("/api/products/{productId}/seo", productSEOHandler.CreateSEO)
		protected.Put("/api/products/{productId}/seo", productSEOHandler.UpdateSEO)
		protected.Delete("/api/products/{productId}/seo", productSEOHandler.DeleteSEO)

		// Connection Credentials endpoints
		protected.Get("/api/connection-credentials", connectionCredentialsHandler.GetCredentials)
		protected.Get("/api/connection-credentials/{id}", connectionCredentialsHandler.GetCredentialByID)
		protected.Post("/api/connection-credentials", connectionCredentialsHandler.CreateCredential)
		protected.Put("/api/connection-credentials/{id}", connectionCredentialsHandler.UpdateCredential)
		protected.Delete("/api/connection-credentials/{id}", connectionCredentialsHandler.DeleteCredential)

		// Connection Settings endpoints
		protected.Get("/api/connection-settings", connectionSettingsHandler.GetSettings)
		protected.Get("/api/connection-settings/{id}", connectionSettingsHandler.GetSettingByID)
		protected.Post("/api/connection-settings", connectionSettingsHandler.CreateSetting)
		protected.Put("/api/connection-settings/{id}", connectionSettingsHandler.UpdateSetting)
		protected.Delete("/api/connection-settings/{id}", connectionSettingsHandler.DeleteSetting)

		// Connection Status endpoints
		protected.Get("/api/connections/{connectionId}/status", connectionStatusHandler.GetStatusByConnection)

		// Channel Parameters endpoints
		protected.Get("/api/channel-parameters", channelParametersHandler.GetParameters)
		protected.Get("/api/channel-parameters/{id}", channelParametersHandler.GetParameterByID)
		protected.Get("/api/channels/{channelId}/parameters", channelParametersHandler.GetParametersByChannel)
		protected.Post("/api/channel-parameters", channelParametersHandler.CreateParameter)
		protected.Put("/api/channel-parameters/{id}", channelParametersHandler.UpdateParameter)
		protected.Delete("/api/channel-parameters/{id}", channelParametersHandler.DeleteParameter)

		// Mercado Libre endpoints
		protected.Get("/api/marketplaces/mercadolibre/category-predictor", mercadoLibreHandler.PredictCategory)
		protected.Post("/api/marketplaces/mercadolibre/products/upload", mercadoLibreHandler.UploadProducts)
		protected.Post("/api/marketplaces/mercadolibre/products/update-price-stock", mercadoLibreHandler.UpdatePricesAndStock)
		protected.Post("/api/marketplaces/mercadolibre/channel-product-map/status", mercadoLibreHandler.RefreshChannelProductMapStatus)
		// Same status refresh as above, plus category drift reconciliation
		// (MercadoLibre auto-recategorizing a listing) — see
		// MercadoLibreProductSyncService.RefreshChannelProductMapStatusAndCategory.
		protected.Post("/api/marketplaces/mercadolibre/channel-product-map/sync", mercadoLibreHandler.RefreshChannelProductMapStatusAndCategory)
		// Reports vehicle compatibility (brand/model/year, plus motor/position/
		// side when available) to MercadoLibre for under_review listings that
		// need it — see MercadoLibreCompatibilitiesHandler.
		protected.Post("/api/marketplaces/mercadolibre/compatibilities/fix-under-review", mercadoLibreCompatibilitiesHandler.FixUnderReview)
		// Sets a MercadoLibre custom attribute value on a product by sku +
		// external_key, resolving/creating the ecom_attributes/
		// ecom_channel_attributes/ecom_channel_attribute_map chain behind it —
		// see channel_attribute_values.Service.SetValue.
		protected.Post("/api/marketplaces/mercadolibre/product-attributes", channelAttributeValueHandler.SetMercadoLibreValue)
		// Seeds the required-attribute slots for a product's MercadoLibre
		// category (sku + MercadoLibre categoryId only) — see
		// channel_attribute_values.Service.ProvisionCategoryAttributes.
		protected.Post("/api/marketplaces/mercadolibre/product-attributes/provision", channelAttributeValueHandler.ProvisionMercadoLibreCategoryAttributes)
		// oauth/authorize is registered as a public route above (not here),
		// since it needs to be reachable without a JWT — see the comment there.

		// Odoo endpoints
		protected.Get("/api/marketplaces/odoo/test-connection", odooHandler.TestConnection)

		// Sources endpoints
		protected.Get("/api/sources", sourceHandler.GetSources)
		protected.Get("/api/sources/{id}", sourceHandler.GetSourceByID)
		protected.Post("/api/sources", sourceHandler.CreateSource)
		protected.Put("/api/sources/{id}", sourceHandler.UpdateSource)
		protected.Delete("/api/sources/{id}", sourceHandler.DeleteSource)

		// Migration endpoints (temporary data-completion helpers, not
		// part of the long-lived marketplace integrations)
		protected.Post("/api/migration/odoo/categories", migrationHandler.SyncOdooCategories)

		// Equipment Types endpoints (machinery compatibility taxonomy)
		protected.Get("/api/equipment-types", equipmentTypeHandler.GetEquipmentTypes)
		protected.Get("/api/equipment-types/{id}", equipmentTypeHandler.GetEquipmentTypeByID)
		protected.Post("/api/equipment-types", equipmentTypeHandler.CreateEquipmentType)
		protected.Put("/api/equipment-types/{id}", equipmentTypeHandler.UpdateEquipmentType)
		protected.Delete("/api/equipment-types/{id}", equipmentTypeHandler.DeleteEquipmentType)

		// Vehicle Fitments endpoints (marca, modelo, año inicio/fin)
		protected.Get("/api/vehicle-fitments", vehicleFitmentHandler.GetVehicleFitments)
		protected.Get("/api/vehicle-fitments/{id}", vehicleFitmentHandler.GetVehicleFitmentByID)
		protected.Post("/api/vehicle-fitments", vehicleFitmentHandler.CreateVehicleFitment)
		protected.Post("/api/vehicle-fitments/bulk-import", vehicleFitmentHandler.BulkImportVehicleFitments)
		protected.Post("/api/vehicle-fitments/resolve-pending", vehicleFitmentHandler.ResolvePendingFitments)
		protected.Put("/api/vehicle-fitments/{id}", vehicleFitmentHandler.UpdateVehicleFitment)
		protected.Delete("/api/vehicle-fitments/{id}", vehicleFitmentHandler.DeleteVehicleFitment)

		// Equipment Fitments endpoints (marca, tipo, modelo y/o serie)
		protected.Get("/api/equipment-fitments", equipmentFitmentHandler.GetEquipmentFitments)
		protected.Get("/api/equipment-fitments/{id}", equipmentFitmentHandler.GetEquipmentFitmentByID)
		protected.Post("/api/equipment-fitments", equipmentFitmentHandler.CreateEquipmentFitment)
		protected.Put("/api/equipment-fitments/{id}", equipmentFitmentHandler.UpdateEquipmentFitment)
		protected.Delete("/api/equipment-fitments/{id}", equipmentFitmentHandler.DeleteEquipmentFitment)

		// Product Vehicle Compatibility endpoints
		protected.Get("/api/products/{productId}/vehicle-compatibilities", productVehicleCompatibilityHandler.GetByProduct)
		protected.Post("/api/product-vehicle-compatibilities", productVehicleCompatibilityHandler.Create)
		protected.Delete("/api/product-vehicle-compatibilities/{id}", productVehicleCompatibilityHandler.Delete)

		// Product Equipment Compatibility endpoints
		protected.Get("/api/products/{productId}/equipment-compatibilities", productEquipmentCompatibilityHandler.GetByProduct)
		protected.Post("/api/product-equipment-compatibilities", productEquipmentCompatibilityHandler.Create)
		protected.Delete("/api/product-equipment-compatibilities/{id}", productEquipmentCompatibilityHandler.Delete)

		// Channel Sync Queue endpoints (marketplace sync waitlist)
		protected.Post("/api/channel-sync-queue", channelSyncQueueHandler.Enqueue)

		// Channel Listings endpoints (create brand-new marketplace listings for
		// a batch of skus, one per vehicle compatibility when the connection
		// allows it — independent from the MercadoLibre upload/update-price-stock
		// endpoints)
		protected.Post("/api/channel-listings/publish", channelListingsHandler.CreateListings)
		protected.Post("/api/channel-listings/refresh", channelListingsHandler.RefreshListings)

		// TEMPORARY debug endpoint — see MercadoLibreItemLookupHandler. Remove
		// once no longer needed.
		protected.Get("/api/marketplaces/mercadolibre/item-lookup", mercadoLibreItemLookupHandler.GetItem)
		protected.Get("/api/marketplaces/mercadolibre/seller-brands", mercadoLibreItemLookupHandler.GetSellerBrands)

		// TEMPORARY one-off migration endpoint — see
		// MercadoLibrePauseUnmappedListingsHandler. Meant to be run once, then
		// removed along with this route.
		protected.Post("/api/marketplaces/mercadolibre/temp-pause-unmapped-listings", mercadoLibrePauseUnmappedListingsHandler.Run)

		// Attributes endpoints (ecom_attributes catalog + ecom_attribute_options
		// enum values + ecom_product_attributes per-product values). CRUD only:
		// not read by any marketplace sync flow yet.
		protected.Get("/api/attributes", attributeHandler.GetAttributes)
		protected.Get("/api/attributes/{id}", attributeHandler.GetAttributeByID)
		protected.Post("/api/attributes", attributeHandler.CreateAttribute)
		protected.Put("/api/attributes/{id}", attributeHandler.UpdateAttribute)
		protected.Delete("/api/attributes/{id}", attributeHandler.DeleteAttribute)

		protected.Get("/api/attributes/{attributeId}/options", attributeOptionHandler.GetOptionsByAttribute)
		protected.Post("/api/attribute-options", attributeOptionHandler.CreateOption)
		protected.Put("/api/attribute-options/{id}", attributeOptionHandler.UpdateOption)
		protected.Delete("/api/attribute-options/{id}", attributeOptionHandler.DeleteOption)

		protected.Get("/api/products/{productId}/attributes", productAttributeHandler.GetAttributesByProduct)
		protected.Put("/api/products/{productId}/attributes", productAttributeHandler.SetAttribute)
		protected.Delete("/api/product-attributes/{id}", productAttributeHandler.DeleteAttribute)

		// Channel Attributes endpoints (ecom_channel_attributes slots +
		// ecom_channel_attribute_map source resolution). CRUD only: replacing
		// the hardcoded MercadoLibre attribute list / missing Odoo
		// attribute_line_ids in sync_mercadolibre_products.go and
		// handler_products.go is a separate, later step.
		protected.Get("/api/channel-attributes", channelAttributeHandler.GetAttributes)
		protected.Get("/api/channel-attributes/{id}", channelAttributeHandler.GetAttributeByID)
		protected.Get("/api/channels/{channelId}/attributes", channelAttributeHandler.GetAttributesByChannel)
		protected.Post("/api/channel-attributes", channelAttributeHandler.CreateAttribute)
		protected.Put("/api/channel-attributes/{id}", channelAttributeHandler.UpdateAttribute)
		protected.Delete("/api/channel-attributes/{id}", channelAttributeHandler.DeleteAttribute)

		protected.Get("/api/channel-attributes/{channelAttributeId}/map", channelAttributeMapHandler.GetMapsByChannelAttribute)
		protected.Post("/api/channel-attribute-map", channelAttributeMapHandler.CreateMap)
		protected.Put("/api/channel-attribute-map/{id}", channelAttributeMapHandler.UpdateMap)
		protected.Delete("/api/channel-attribute-map/{id}", channelAttributeMapHandler.DeleteMap)
	})

	return r
}
