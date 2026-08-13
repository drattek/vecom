package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"

	"github.com/joho/godotenv"

	"core-orchestrator/internal/api"
	attributesApp "core-orchestrator/internal/application/attributes"
	brandsApp "core-orchestrator/internal/application/brands"
	categoriesApp "core-orchestrator/internal/application/categories"
	channelAttributeValuesApp "core-orchestrator/internal/application/channel_attribute_values"
	channelAttributesApp "core-orchestrator/internal/application/channel_attributes"
	channelConfigApp "core-orchestrator/internal/application/channel_config"
	channelConnectionsService "core-orchestrator/internal/application/channel_connections"
	channelListingsApp "core-orchestrator/internal/application/channel_listings"
	channelSyncQueueApp "core-orchestrator/internal/application/channel_sync_queue"
	channelsService "core-orchestrator/internal/application/channels"
	compatibilityApp "core-orchestrator/internal/application/compatibility"
	credService "core-orchestrator/internal/application/credentials"
	currenciesApp "core-orchestrator/internal/application/currencies"
	filesService "core-orchestrator/internal/application/files"
	inventoryApp "core-orchestrator/internal/application/inventory"
	meliNotificationsApp "core-orchestrator/internal/application/meli_notifications"
	migrationApp "core-orchestrator/internal/application/migration"
	pricingApp "core-orchestrator/internal/application/pricing"
	productImageImportApp "core-orchestrator/internal/application/product_image_import"
	productMediaApp "core-orchestrator/internal/application/product_media"
	prodService "core-orchestrator/internal/application/products"
	sourcesApp "core-orchestrator/internal/application/sources"
	stockMovementsApp "core-orchestrator/internal/application/stock_movements"
	storageDisksService "core-orchestrator/internal/application/storage_disks"
	syncApp "core-orchestrator/internal/application/sync"
	"core-orchestrator/internal/config"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	odooInfra "core-orchestrator/internal/infrastructure/marketplace/odoo"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	redisInfra "core-orchestrator/internal/infrastructure/redis"
	serverTLS "core-orchestrator/internal/infrastructure/servertls"
	httpHandler "core-orchestrator/internal/interfaces/http"
	"core-orchestrator/internal/interfaces/schedulers"
	"core-orchestrator/internal/workers"
)

func main() {
	// Load .env file
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	cfg := config.Load()

	// Initialize database connection
	db := mysqlInfra.NewConnection()
	defer db.Close()

	// Initialize MySQL repositories
	mysqlRepos := initializeMySQLRepositories(db)

	// Initialize services
	productService := prodService.NewProductService(mysqlRepos.ProductRepository, mysqlRepos.PendingProductVehicleFitmentsRepository, mysqlRepos.ProductVehicleCompatibilityRepository, mysqlRepos.PartNumberSupersessionsRepository)
	storageDiskService := storageDisksService.NewStorageDiskService(mysqlRepos.StorageDiskRepository)
	fileService := filesService.NewFileService(mysqlRepos.FilesRepository)
	channelService := channelsService.NewChannelService(mysqlRepos.ChannelRepository)
	channelConnectionService := channelConnectionsService.NewChannelConnectionService(mysqlRepos.ChannelConnectionRepository)
	authService := credService.NewAuthService(
		mysqlRepos.AuthRepository,
		cfg.JWTSecret,
		cfg.JWTIssuer,
		cfg.JWTTTLMinutes,
	)
	brandService := brandsApp.NewBrandService(mysqlRepos.BrandsRepository)
	categoryService := categoriesApp.NewCategoryService(mysqlRepos.CategoriesRepository)
	currencyService := currenciesApp.NewCurrencyService(mysqlRepos.CurrenciesRepository)
	sourceService := sourcesApp.NewSourceService(mysqlRepos.SourcesRepository)
	branchService := inventoryApp.NewBranchService(mysqlRepos.BranchesRepository)
	warehouseService := inventoryApp.NewWarehouseService(mysqlRepos.WarehousesRepository)
	productStockService := inventoryApp.NewProductStockService(mysqlRepos.ProductStockRepository)
	priceListService := pricingApp.NewPriceListService(mysqlRepos.PriceListRepository)
	productPricesService := pricingApp.NewProductPricesService(mysqlRepos.ProductPricesRepository)
	exchangeRatesService := pricingApp.NewExchangeRatesService(mysqlRepos.ExchangeRatesRepository)
	stockMovementsService := stockMovementsApp.NewStockMovementsService(mysqlRepos.StockMovementsRepository)
	productImagesService := productMediaApp.NewProductImagesService(mysqlRepos.ProductImagesRepository)
	productImageImportService := productImageImportApp.NewService(
		mysqlRepos.ProductRepository,
		mysqlRepos.FilesRepository,
		mysqlRepos.ProductImagesRepository,
		mysqlRepos.StorageDiskRepository,
	)
	productVideosService := productMediaApp.NewProductVideosService(mysqlRepos.ProductVideosRepository)
	productPartNumbersService := productMediaApp.NewProductPartNumbersService(mysqlRepos.ProductPartNumbersRepository)
	productDimensionsService := productMediaApp.NewProductDimensionsService(mysqlRepos.ProductDimensionsRepository, mysqlRepos.ProductRepository)
	productSEOService := productMediaApp.NewProductSEOService(mysqlRepos.ProductSEORepository)
	connectionCredentialsService := channelConfigApp.NewConnectionCredentialsService(mysqlRepos.ConnectionCredentialsRepository)
	connectionSettingsService := channelConfigApp.NewConnectionSettingsService(mysqlRepos.ConnectionSettingsRepository)
	connectionStatusService := channelConfigApp.NewConnectionStatusService(mysqlRepos.ConnectionStatusRepository)
	channelParametersService := channelConfigApp.NewChannelParametersService(mysqlRepos.ChannelParametersRepository)
	// Attributes: catalog of product attributes (ecom_attributes), their
	// enum value lists (ecom_attribute_options) and per-product values
	// (ecom_product_attributes). Not wired into any marketplace sync flow
	// yet — the tables are still being populated with data.
	attributeService := attributesApp.NewAttributeService(mysqlRepos.AttributesRepository)
	attributeOptionService := attributesApp.NewAttributeOptionService(mysqlRepos.AttributeOptionsRepository)
	productAttributeService := attributesApp.NewProductAttributeService(mysqlRepos.ProductAttributesRepository, mysqlRepos.AttributesRepository)
	// Channel attributes: catalog of attribute "slots" each channel exposes
	// (ecom_channel_attributes) and which internal source fills each one
	// (ecom_channel_attribute_map). Same as above — CRUD only for now, not
	// consumed by sync_mercadolibre_products.go/handler_products.go.
	channelAttributeService := channelAttributesApp.NewChannelAttributeService(mysqlRepos.ChannelAttributesRepository)
	channelAttributeMapService := channelAttributesApp.NewChannelAttributeMapService(mysqlRepos.ChannelAttributeMapRepository)
	meliNotificationService := meliNotificationsApp.NewMeliNotificationService(mysqlRepos.MeliNotificationRepository)
	// Shared across every MercadoLibre client so the 10-calls-per-minute
	// budget is enforced globally (auth, categories, etc. all draw from the
	// same pool), not reset per connection or per call site.
	mercadoLibreRateLimiter := mercadoLibreInfra.NewRateLimiter(syncApp.MercadoLibreRateLimit, syncApp.MercadoLibreRateLimitWindow)
	mercadoLibreTokenService := syncApp.NewMercadoLibreTokenService(mysqlRepos.ConnectionSettingsRepository, mysqlRepos.ConnectionStatusRepository, mercadoLibreRateLimiter)
	mercadoLibreCategoryPredictorService := syncApp.NewMercadoLibreCategoryPredictorService(
		mercadoLibreTokenService,
		mysqlRepos.ConnectionSettingsRepository,
		mysqlRepos.CategoriesRepository,
		mysqlRepos.ChannelCategoryMapRepository,
		mercadoLibreRateLimiter,
	)
	// Composes the four services/repositories above to set a MercadoLibre
	// custom attribute value from just {sku, externalKey, value}, or to
	// provision a whole category's required attributes from just
	// {sku, categoryId} — see channel_attribute_values.Service.
	channelAttributeValuesService := channelAttributeValuesApp.NewService(
		mysqlRepos.ProductRepository,
		mysqlRepos.ChannelRepository,
		mysqlRepos.AttributesRepository,
		mysqlRepos.AttributeOptionsRepository,
		mysqlRepos.ChannelAttributesRepository,
		mysqlRepos.ChannelAttributeMapRepository,
		productAttributeService,
		mercadoLibreInfra.NewCategoriesHandler(mercadoLibreInfra.NewClient(nil, "", mercadoLibreRateLimiter)),
		mercadoLibreCategoryPredictorService,
	)
	mercadoLibreCompatibilityService := syncApp.NewMercadoLibreCompatibilityService(
		mysqlRepos.ChannelProductMapRepository,
		mysqlRepos.ChannelConnectionRepository,
		mysqlRepos.ChannelRepository,
		mysqlRepos.ProductRepository,
		mysqlRepos.VehicleFitmentsRepository,
		mysqlRepos.BrandsRepository,
		mysqlRepos.ProductVehicleCompatibilityRepository,
		mercadoLibreTokenService,
		mercadoLibreRateLimiter,
	)
	// Scans every listing in a MercadoLibre account for the flagged
	// price=999999/stock=0 sentinel and downloads/mirrors it locally — see
	// sync.MercadoLibreListingsAuditService.
	mercadoLibreListingsAuditService := syncApp.NewMercadoLibreListingsAuditService(
		mysqlRepos.ChannelConnectionRepository,
		mysqlRepos.ChannelRepository,
		mysqlRepos.ProductRepository,
		mercadoLibreTokenService,
		channelAttributeValuesService,
		mercadoLibreRateLimiter,
	)
	mercadoLibreProductSyncService := syncApp.NewMercadoLibreProductSyncService(
		mysqlRepos.ProductRepository,
		mysqlRepos.BrandsRepository,
		mysqlRepos.ProductPricesRepository,
		mysqlRepos.ProductStockRepository,
		mysqlRepos.ProductImagesRepository,
		mysqlRepos.ProductDimensionsRepository,
		mysqlRepos.FilesRepository,
		mysqlRepos.CurrenciesRepository,
		mysqlRepos.ChannelConnectionRepository,
		mysqlRepos.ChannelProductMapRepository,
		mysqlRepos.ChannelCategoryMapRepository,
		mercadoLibreTokenService,
		mercadoLibreCategoryPredictorService,
		mercadoLibreRateLimiter,
		channelAttributeValuesService,
		mysqlRepos.ProductAttributesRepository,
		mysqlRepos.AttributeOptionsRepository,
	)
	// Shared across every Odoo client so the 10-calls-per-minute budget is
	// enforced globally (products, categories, etc. all draw from the same
	// pool), not reset per connection or per call site.
	odooRateLimiter := odooInfra.NewRateLimiter(syncApp.OdooRateLimit, syncApp.OdooRateLimitWindow)
	odooConnectionService := syncApp.NewOdooConnectionService(
		mysqlRepos.ConnectionCredentialsRepository,
		mysqlRepos.ConnectionSettingsRepository,
		mysqlRepos.ConnectionStatusRepository,
		odooRateLimiter,
	)
	odooCategoryMigrationService := migrationApp.NewOdooCategoryMigrationService(
		mysqlRepos.ConnectionCredentialsRepository,
		mysqlRepos.ConnectionSettingsRepository,
		mysqlRepos.CategoriesRepository,
		mysqlRepos.ChannelCategoryMapRepository,
		odooRateLimiter,
	)
	odooProductSyncService := syncApp.NewOdooProductSyncService(
		mysqlRepos.ConnectionCredentialsRepository,
		mysqlRepos.ConnectionSettingsRepository,
		mysqlRepos.ProductRepository,
		mysqlRepos.ProductPricesRepository,
		mysqlRepos.ProductStockRepository,
		mysqlRepos.ProductDimensionsRepository,
		mysqlRepos.ProductImagesRepository,
		mysqlRepos.FilesRepository,
		mysqlRepos.CurrenciesRepository,
		mysqlRepos.CategoriesRepository,
		mysqlRepos.ChannelCategoryMapRepository,
		mysqlRepos.ChannelProductMapRepository,
		odooRateLimiter,
	)
	equipmentTypeService := compatibilityApp.NewEquipmentTypeService(mysqlRepos.EquipmentTypesRepository)
	vehicleFitmentService := compatibilityApp.NewVehicleFitmentService(mysqlRepos.VehicleFitmentsRepository, mysqlRepos.BrandsRepository, mysqlRepos.ProductRepository, mysqlRepos.ProductVehicleCompatibilityRepository, mysqlRepos.PendingProductVehicleFitmentsRepository)
	equipmentFitmentService := compatibilityApp.NewEquipmentFitmentService(mysqlRepos.EquipmentFitmentsRepository)
	productVehicleCompatibilityService := compatibilityApp.NewProductVehicleCompatibilityService(mysqlRepos.ProductVehicleCompatibilityRepository)
	productEquipmentCompatibilityService := compatibilityApp.NewProductEquipmentCompatibilityService(mysqlRepos.ProductEquipmentCompatibilityRepository)
	channelSyncQueueService := channelSyncQueueApp.NewService(
		mysqlRepos.ProductRepository,
		mysqlRepos.ChannelConnectionRepository,
		mysqlRepos.ChannelSyncQueueRepository,
	)
	// Channel listings: creates brand-new marketplace listings for a batch of
	// skus (one per vehicle compatibility when the connection allows it),
	// dispatching to whichever publisher is registered for the connection's
	// channel code. Registering additional marketplaces later only means
	// adding another Register call here.
	channelListingsService := channelListingsApp.NewService(
		mysqlRepos.ProductRepository,
		mysqlRepos.ProductImagesRepository,
		mysqlRepos.ProductStockRepository,
		mysqlRepos.BrandsRepository,
		mysqlRepos.ChannelRepository,
		mysqlRepos.ChannelConnectionRepository,
		mysqlRepos.ChannelProductMapRepository,
		mysqlRepos.ChannelSyncQueueRepository,
		mysqlRepos.ProductVehicleCompatibilityRepository,
		mysqlRepos.VehicleFitmentsRepository,
		mysqlRepos.PartNumberSupersessionsRepository,
	)
	channelListingsService.Register("MERCADOLIBRE", mercadoLibreProductSyncService)
	channelListingsService.Register("ODOO", odooProductSyncService)
	channelListingsService.RegisterRefresher("MERCADOLIBRE", mercadoLibreProductSyncService)
	channelListingsService.RegisterRefresher("ODOO", odooProductSyncService)

	productHandler := httpHandler.NewProductHandler(productService)
	storageDiskHandler := httpHandler.NewStorageDiskHandler(storageDiskService)
	fileHandler := httpHandler.NewFileHandler(fileService)
	channelHandler := httpHandler.NewChannelHandler(channelService)
	channelConnectionHandler := httpHandler.NewChannelConnectionHandler(channelConnectionService)
	authHandler := httpHandler.NewAuthHandler(authService)
	authMiddleware := httpHandler.NewJWTMiddleware(authService)
	brandHandler := httpHandler.NewBrandHandler(brandService)
	categoryHandler := httpHandler.NewCategoryHandler(categoryService)
	currencyHandler := httpHandler.NewCurrencyHandler(currencyService)
	sourceHandler := httpHandler.NewSourceHandler(sourceService)
	branchHandler := httpHandler.NewBranchHandler(branchService)
	warehouseHandler := httpHandler.NewWarehouseHandler(warehouseService)
	productStockHandler := httpHandler.NewProductStockHandler(productStockService)
	priceListHandler := httpHandler.NewPriceListHandler(priceListService)
	productPricesHandler := httpHandler.NewProductPricesHandler(productPricesService)
	exchangeRatesHandler := httpHandler.NewExchangeRatesHandler(exchangeRatesService)
	stockMovementsHandler := httpHandler.NewStockMovementsHandler(stockMovementsService)
	productImagesHandler := httpHandler.NewProductImagesHandler(productImagesService)
	productImageImportHandler := httpHandler.NewProductImageImportHandler(productImageImportService)
	productVideosHandler := httpHandler.NewProductVideosHandler(productVideosService)
	productPartNumbersHandler := httpHandler.NewProductPartNumbersHandler(productPartNumbersService)
	productDimensionsHandler := httpHandler.NewProductDimensionsHandler(productDimensionsService)
	productSEOHandler := httpHandler.NewProductSEOHandler(productSEOService)
	connectionCredentialsHandler := httpHandler.NewConnectionCredentialsHandler(connectionCredentialsService)
	connectionSettingsHandler := httpHandler.NewConnectionSettingsHandler(connectionSettingsService)
	connectionStatusHandler := httpHandler.NewConnectionStatusHandler(connectionStatusService)
	channelParametersHandler := httpHandler.NewChannelParametersHandler(channelParametersService)
	attributeHandler := httpHandler.NewAttributeHandler(attributeService)
	attributeOptionHandler := httpHandler.NewAttributeOptionHandler(attributeOptionService)
	productAttributeHandler := httpHandler.NewProductAttributeHandler(productAttributeService)
	channelAttributeHandler := httpHandler.NewChannelAttributeHandler(channelAttributeService)
	channelAttributeMapHandler := httpHandler.NewChannelAttributeMapHandler(channelAttributeMapService)
	channelAttributeValueHandler := httpHandler.NewChannelAttributeValueHandler(channelAttributeValuesService)
	mercadoLibreHandler := httpHandler.NewMercadoLibreHandler(mercadoLibreCategoryPredictorService, mercadoLibreProductSyncService, mercadoLibreTokenService)
	mercadoLibreCompatibilitiesHandler := httpHandler.NewMercadoLibreCompatibilitiesHandler(mercadoLibreCompatibilityService)
	mercadoLibreListingsAuditHandler := httpHandler.NewMercadoLibreListingsAuditHandler(mercadoLibreListingsAuditService)
	// TEMPORARY debug endpoint — remove once the attribute-provisioning
	// investigation it's for is done.
	mercadoLibreCategoryAttributesDebugHandler := httpHandler.NewMercadoLibreCategoryAttributesDebugHandler(mercadoLibreCategoryPredictorService)
	meliNotificationHandler := httpHandler.NewMeliNotificationHandler(meliNotificationService)
	odooHandler := httpHandler.NewOdooHandler(odooConnectionService)
	migrationHandler := httpHandler.NewMigrationHandler(odooCategoryMigrationService)
	equipmentTypeHandler := httpHandler.NewEquipmentTypeHandler(equipmentTypeService)
	vehicleFitmentHandler := httpHandler.NewVehicleFitmentHandler(vehicleFitmentService)
	equipmentFitmentHandler := httpHandler.NewEquipmentFitmentHandler(equipmentFitmentService)
	productVehicleCompatibilityHandler := httpHandler.NewProductVehicleCompatibilityHandler(productVehicleCompatibilityService)
	productEquipmentCompatibilityHandler := httpHandler.NewProductEquipmentCompatibilityHandler(productEquipmentCompatibilityService)
	channelSyncQueueHandler := httpHandler.NewChannelSyncQueueHandler(channelSyncQueueService)
	channelListingsHandler := httpHandler.NewChannelListingsHandler(channelListingsService)
	// TEMPORARY debug handler — see MercadoLibreItemLookupHandler. Remove
	// once no longer needed.
	mercadoLibreItemLookupHandler := httpHandler.NewMercadoLibreItemLookupHandler(mercadoLibreTokenService)
	// TEMPORARY one-off migration handler — see
	// MercadoLibrePauseUnmappedListingsHandler. Meant to be run once, then
	// removed.
	mercadoLibrePauseUnmappedListingsHandler := httpHandler.NewMercadoLibrePauseUnmappedListingsHandler(mercadoLibreTokenService, mysqlRepos.ChannelProductMapRepository, mercadoLibreRateLimiter)

	// Initialize Redis and sync service
	redisClient := redisInfra.NewClient()
	redisProductRepo := redisInfra.NewProductRepository(redisClient)
	redisInventoryRepo := redisInfra.NewInventoryRepository(redisClient)
	redisNissanRepo := redisInfra.NewNissanRepository(redisClient)
	syncService := syncApp.NewSyncService(redisProductRepo, redisInventoryRepo, redisNissanRepo, db)

	// Start consumers in background so API availability is not blocked.
	go func() {
		if consumerErr := startConsumers(syncService); consumerErr != nil {
			log.Printf("Failed to start consumers: %v", consumerErr)
		}
	}()

	// Marketplace sync worker: polls ecom_channel_sync_queue for every
	// pending entry, regardless of who queued it. "listing" entries (queued
	// by channelListingsService itself, after a stock/cover-image
	// precondition failed on a publish request) are retried through it,
	// recomputing stock/image/vehicle-compatibility fan-out live; every
	// other entry (from the channel-agnostic /api/channel-sync-queue
	// endpoint) is dispatched to the ProductSyncer registered for its
	// connection's channel code, same as before. Registering additional
	// marketplaces (MercadoLibre, Amazon, ...) on that second path only
	// means adding another Register call here.
	marketplaceWorker := workers.NewMarketplaceWorker(
		mysqlRepos.ChannelSyncQueueRepository,
		mysqlRepos.ChannelConnectionRepository,
		mysqlRepos.ChannelRepository,
		channelListingsService,
		workers.MarketplaceWorkerConfig{
			PollInterval: cfg.SyncQueuePollInterval,
			BatchSize:    cfg.SyncQueueBatchSize,
		},
	)
	marketplaceWorker.Register("ODOO", odooProductSyncService)
	go marketplaceWorker.Start(context.Background())

	// Token refresh scheduler: polls ecom_connection_status for connections
	// whose token is close to expiring and delegates the refresh to whichever
	// TokenRefresher is registered for that connection's channel code. Odoo
	// needs no entry here — it's API-key based and never sets expires_at, so
	// FindDueForRefresh never surfaces it. Registering additional marketplaces
	// (Amazon, ...) later only means adding another Register call here.
	tokenRefreshScheduler := schedulers.NewTokenRefreshScheduler(
		mysqlRepos.ConnectionStatusRepository,
		schedulers.TokenRefreshSchedulerConfig{
			PollInterval:    cfg.TokenRefreshPollInterval,
			ExpiryLookahead: cfg.TokenRefreshLookahead,
		},
	)
	tokenRefreshScheduler.Register("MERCADOLIBRE", mercadoLibreTokenService)
	go tokenRefreshScheduler.Start(context.Background())

	// Compatibilities fix scheduler: once a day calls
	// mercadoLibreCompatibilityService.FixUnderReviewListings for every
	// active MERCADOLIBRE connection, so a listing stuck under_review for
	// missing vehicle compatibilities gets fixed automatically instead of
	// depending on someone calling the manual fix-under-review endpoint.
	compatibilitiesFixScheduler := schedulers.NewCompatibilitiesFixScheduler(
		mysqlRepos.ChannelConnectionRepository,
		mercadoLibreCompatibilityService,
		schedulers.CompatibilitiesFixSchedulerConfig{
			RunAtHour:   cfg.CompatibilitiesFixRunAtHour,
			RunAtMinute: cfg.CompatibilitiesFixRunAtMinute,
		},
	)
	go compatibilitiesFixScheduler.Start(context.Background())

	// Create router and start server.
	router := api.NewRouter(
		productHandler, storageDiskHandler, fileHandler, channelHandler, channelConnectionHandler,
		authHandler, authMiddleware, brandHandler, categoryHandler, currencyHandler, branchHandler,
		warehouseHandler, productStockHandler, priceListHandler, productPricesHandler, exchangeRatesHandler,
		stockMovementsHandler, productImagesHandler, productImageImportHandler, productVideosHandler, productPartNumbersHandler,
		productDimensionsHandler, productSEOHandler, connectionCredentialsHandler, connectionSettingsHandler,
		connectionStatusHandler, channelParametersHandler, mercadoLibreHandler, meliNotificationHandler, odooHandler, sourceHandler,
		migrationHandler, equipmentTypeHandler, vehicleFitmentHandler, equipmentFitmentHandler,
		productVehicleCompatibilityHandler, productEquipmentCompatibilityHandler, channelSyncQueueHandler,
		channelListingsHandler, mercadoLibreItemLookupHandler, mercadoLibrePauseUnmappedListingsHandler,
		attributeHandler, attributeOptionHandler, productAttributeHandler, channelAttributeHandler, channelAttributeMapHandler,
		channelAttributeValueHandler,
		mercadoLibreCompatibilitiesHandler,
		mercadoLibreListingsAuditHandler,
		mercadoLibreCategoryAttributesDebugHandler,
	)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	tlsConfig, err := serverTLS.LoadTLSConfig(cfg.TLSKeystorePath, cfg.TLSKeystorePassword, cfg.TLSKeystoreType)
	if err != nil {
		log.Fatalf("Failed to configure TLS: %v", err)
	}

	if tlsConfig != nil {
		server.TLSConfig = tlsConfig
		listener, listenErr := tls.Listen("tcp", server.Addr, tlsConfig)
		if listenErr != nil {
			log.Fatalf("Failed to start HTTPS server: %v", listenErr)
		}

		log.Printf("HTTPS server starting on port %s...", cfg.Port)
		log.Fatal(server.Serve(listener))
	}

	log.Printf("HTTP server starting on port %s...", cfg.Port)
	log.Fatal(server.ListenAndServe())
}
