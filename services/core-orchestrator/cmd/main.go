package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	productAttributeChecklistApp "core-orchestrator/internal/application/product_attributes_checklist"
	productDetailsApp "core-orchestrator/internal/application/product_details"
	productImageImportApp "core-orchestrator/internal/application/product_image_import"
	productMediaApp "core-orchestrator/internal/application/product_media"
	prodService "core-orchestrator/internal/application/products"
	sourcesApp "core-orchestrator/internal/application/sources"
	stockMovementsApp "core-orchestrator/internal/application/stock_movements"
	storageDisksService "core-orchestrator/internal/application/storage_disks"
	syncApp "core-orchestrator/internal/application/sync"
	"core-orchestrator/internal/config"
	banxicoInfra "core-orchestrator/internal/infrastructure/banxico"
	mercadoLibreInfra "core-orchestrator/internal/infrastructure/marketplace/mercadolibre"
	odooInfra "core-orchestrator/internal/infrastructure/marketplace/odoo"
	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
	redisInfra "core-orchestrator/internal/infrastructure/redis"
	serverTLS "core-orchestrator/internal/infrastructure/servertls"
	httpHandler "core-orchestrator/internal/interfaces/http"
	"core-orchestrator/internal/interfaces/schedulers"
	"core-orchestrator/internal/shared/safe"
	"core-orchestrator/internal/workers"
)

// consumersShutdownGrace acota cuánto espera el apagado ordenado a que los
// consumers RabbitMQ terminen el mensaje en curso. Los handlers no reciben
// context, así que no se pueden cancelar; agotado el plazo el proceso sale
// igual y RabbitMQ reencola lo que quedó sin ack.
const consumersShutdownGrace = 10 * time.Second

func main() {
	// Load .env file
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	cfg := config.Load()

	// rootCtx se cancela al recibir SIGINT/SIGTERM (deploy/restart). Dispara el
	// cierre ordenado del servidor HTTP más abajo.
	rootCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// bgCtx gobierna los procesos de fondo (worker de sync a marketplaces,
	// schedulers). Se cancela recién DESPUÉS de que server.Shutdown drenó las
	// requests en vuelo, para no cortar una operación de marketplace que una
	// request HTTP todavía en curso haya lanzado.
	bgCtx, stopBackground := context.WithCancel(context.Background())
	defer stopBackground()

	// Initialize database connection
	db := mysqlInfra.NewConnection(cfg)
	defer db.Close()

	// Initialize MySQL repositories
	mysqlRepos := initializeMySQLRepositories(db)

	// Initialize services
	productService := prodService.NewProductService(db, mysqlRepos.ProductRepository, mysqlRepos.PendingProductVehicleFitmentsRepository, mysqlRepos.ProductVehicleCompatibilityRepository, mysqlRepos.PartNumberSupersessionsRepository)
	storageDiskService := storageDisksService.NewStorageDiskService(db, mysqlRepos.StorageDiskRepository)
	fileService := filesService.NewFileService(db, mysqlRepos.FilesRepository)
	channelService := channelsService.NewChannelService(db, mysqlRepos.ChannelRepository, mysqlRepos.ChannelProductMapRepository)
	channelConnectionService := channelConnectionsService.NewChannelConnectionService(db, mysqlRepos.ChannelConnectionRepository)
	authService := credService.NewAuthService(
		db,
		mysqlRepos.AuthRepository,
		cfg.JWTSecret,
		cfg.JWTIssuer,
		cfg.JWTTTLMinutes,
	)
	brandService := brandsApp.NewBrandService(db, mysqlRepos.BrandsRepository, mysqlRepos.ProductRepository)
	categoryService := categoriesApp.NewCategoryService(db, mysqlRepos.CategoriesRepository)
	currencyService := currenciesApp.NewCurrencyService(db, mysqlRepos.CurrenciesRepository)
	sourceService := sourcesApp.NewSourceService(db, mysqlRepos.SourcesRepository)
	branchService := inventoryApp.NewBranchService(db, mysqlRepos.BranchesRepository)
	warehouseService := inventoryApp.NewWarehouseService(db, mysqlRepos.WarehousesRepository)
	productStockService := inventoryApp.NewProductStockService(db, mysqlRepos.ProductStockRepository)
	priceListService := pricingApp.NewPriceListService(db, mysqlRepos.PriceListRepository)
	pricingFormulaService := pricingApp.NewPricingFormulaService(db, mysqlRepos.PricingFormulaRepository)
	pricingFormulaCalculator := pricingApp.NewPricingFormulaCalculator(mysqlRepos.PricingFormulaRepository)
	effectivePriceResolver := pricingApp.NewEffectivePriceResolver(mysqlRepos.ProductPricesRepository, mysqlRepos.ExchangeRatesRepository)
	productPricesService := pricingApp.NewProductPricesService(db, mysqlRepos.ProductPricesRepository, mysqlRepos.ProductRepository, mysqlRepos.PriceListRepository)
	exchangeRatesService := pricingApp.NewExchangeRatesService(db, mysqlRepos.ExchangeRatesRepository)
	sieClient := banxicoInfra.NewClient(nil, cfg.SIEAPIToken)
	sieExchangeRateUpdater := pricingApp.NewSIEExchangeRateUpdater(sieClient, mysqlRepos.CurrenciesRepository, mysqlRepos.ExchangeRatesRepository)
	stockMovementsService := stockMovementsApp.NewStockMovementsService(db, mysqlRepos.StockMovementsRepository)
	productImagesService := productMediaApp.NewProductImagesService(db, mysqlRepos.ProductImagesRepository)
	// Read/write model de la página de detalle de producto (lecturas por
	// sección + edición inline de la sección General vía PATCH).
	productDetailsService := productDetailsApp.NewService(
		db,
		mysqlRepos.ProductDetailsRepository,
		mysqlRepos.BrandsRepository,
		mysqlRepos.CategoriesRepository,
		mysqlRepos.ProductDimensionsRepository,
		mysqlRepos.ProductSEORepository,
	)
	productAttributeChecklistService := productAttributeChecklistApp.NewService(
		mysqlRepos.ProductDetailsRepository,
		mysqlRepos.ChannelConnectionRepository,
		mysqlRepos.ChannelCategoryMapRepository,
		mysqlRepos.ChannelAttributesRepository,
		mysqlRepos.ChannelAttributeMapRepository,
		mysqlRepos.AttributesRepository,
		mysqlRepos.AttributeOptionsRepository,
		mysqlRepos.ProductAttributesRepository,
	)
	productImageImportService := productImageImportApp.NewService(
		db,
		mysqlRepos.ProductRepository,
		mysqlRepos.FilesRepository,
		mysqlRepos.ProductImagesRepository,
		mysqlRepos.ProductVideosRepository,
		mysqlRepos.ProductMediaRepository,
		mysqlRepos.StorageDiskRepository,
	)
	productVideosService := productMediaApp.NewProductVideosService(db, mysqlRepos.ProductVideosRepository)
	productPartNumbersService := productMediaApp.NewProductPartNumbersService(db, mysqlRepos.ProductPartNumbersRepository)
	productDimensionsService := productMediaApp.NewProductDimensionsService(db, mysqlRepos.ProductDimensionsRepository, mysqlRepos.ProductRepository)
	productSEOService := productMediaApp.NewProductSEOService(db, mysqlRepos.ProductSEORepository)
	connectionCredentialsService := channelConfigApp.NewConnectionCredentialsService(db, mysqlRepos.ConnectionCredentialsRepository)
	connectionSettingsService := channelConfigApp.NewConnectionSettingsService(db, mysqlRepos.ConnectionSettingsRepository)
	connectionStatusService := channelConfigApp.NewConnectionStatusService(db, mysqlRepos.ConnectionStatusRepository)
	channelParametersService := channelConfigApp.NewChannelParametersService(db, mysqlRepos.ChannelParametersRepository)
	// Attributes: catalog of product attributes (ecom_attributes), their
	// enum value lists (ecom_attribute_options) and per-product values
	// (ecom_product_attributes). Not wired into any marketplace sync flow
	// yet — the tables are still being populated with data.
	attributeService := attributesApp.NewAttributeService(db, mysqlRepos.AttributesRepository)
	attributeOptionService := attributesApp.NewAttributeOptionService(db, mysqlRepos.AttributeOptionsRepository)
	productAttributeService := attributesApp.NewProductAttributeService(db, mysqlRepos.ProductAttributesRepository, mysqlRepos.AttributesRepository)
	// Channel attributes: catalog of attribute "slots" each channel exposes
	// (ecom_channel_attributes) and which internal source fills each one
	// (ecom_channel_attribute_map). Same as above — CRUD only for now, not
	// consumed by sync_mercadolibre_products.go/handler_products.go.
	channelAttributeService := channelAttributesApp.NewChannelAttributeService(db, mysqlRepos.ChannelAttributesRepository)
	channelAttributeMapService := channelAttributesApp.NewChannelAttributeMapService(db, mysqlRepos.ChannelAttributeMapRepository)
	meliNotificationService := meliNotificationsApp.NewMeliNotificationService(db, mysqlRepos.MeliNotificationRepository)
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
		db,
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
	// price=999999/stock=0 sentinel and, per match, resolves/creates the local
	// product and copies the listing's vehicle compatibilities into MySQL —
	// see sync.MercadoLibreListingsAuditService (reuses
	// mercadoLibreCompatibilityService for the copy).
	mercadoLibreListingsAuditService := syncApp.NewMercadoLibreListingsAuditService(
		mysqlRepos.ChannelConnectionRepository,
		mysqlRepos.ChannelRepository,
		mysqlRepos.ProductRepository,
		mercadoLibreTokenService,
		mercadoLibreCompatibilityService,
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
		pricingFormulaCalculator,
		effectivePriceResolver,
		mercadoLibreCompatibilityService,
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
	// TEMPORARY one-off migration service — migra vecom_sync_product /
	// vecom_products (sistema anterior) a ecom_products / ecom_channel_product_map.
	// Eliminar junto con su handler y su ruta al terminar la migración.
	vecomSyncProductMigrationService := migrationApp.NewVecomSyncProductMigrationService(
		db,
		mysqlRepos.ProductRepository,
		mysqlRepos.BrandsRepository,
		mysqlRepos.ProductDimensionsRepository,
		mysqlRepos.ChannelProductMapRepository,
		mysqlRepos.ChannelCategoryMapRepository,
		mysqlRepos.CategoriesRepository,
		mysqlRepos.ConnectionCredentialsRepository,
		mysqlRepos.ConnectionSettingsRepository,
		mercadoLibreTokenService,
		channelAttributeValuesService,
		mercadoLibreRateLimiter,
		odooRateLimiter,
	)
	// TEMPORARY one-off migration service — migra vecom_images (sistema
	// anterior) a ecom_files / ecom_product_images, vinculando por
	// vecom_products.code = ecom_products.sku. Eliminar junto con su handler
	// y su ruta al terminar la migración.
	vecomImagesMigrationService := migrationApp.NewVecomImagesMigrationService(
		db,
		mysqlRepos.ProductRepository,
		mysqlRepos.FilesRepository,
		mysqlRepos.ProductImagesRepository,
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
		pricingFormulaCalculator,
		effectivePriceResolver,
	)
	equipmentTypeService := compatibilityApp.NewEquipmentTypeService(db, mysqlRepos.EquipmentTypesRepository)
	vehicleFitmentService := compatibilityApp.NewVehicleFitmentService(db, mysqlRepos.VehicleFitmentsRepository, mysqlRepos.BrandsRepository, mysqlRepos.ProductRepository, mysqlRepos.ProductVehicleCompatibilityRepository, mysqlRepos.PendingProductVehicleFitmentsRepository)
	equipmentFitmentService := compatibilityApp.NewEquipmentFitmentService(db, mysqlRepos.EquipmentFitmentsRepository)
	productVehicleCompatibilityService := compatibilityApp.NewProductVehicleCompatibilityService(db, mysqlRepos.ProductVehicleCompatibilityRepository)
	productEquipmentCompatibilityService := compatibilityApp.NewProductEquipmentCompatibilityService(db, mysqlRepos.ProductEquipmentCompatibilityRepository)
	channelSyncQueueService := channelSyncQueueApp.NewService(
		db,
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
		db,
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
	pricingFormulaHandler := httpHandler.NewPricingFormulaHandler(pricingFormulaService)
	productPricesHandler := httpHandler.NewProductPricesHandler(productPricesService)
	exchangeRatesHandler := httpHandler.NewExchangeRatesHandler(exchangeRatesService)
	stockMovementsHandler := httpHandler.NewStockMovementsHandler(stockMovementsService)
	productImagesHandler := httpHandler.NewProductImagesHandler(productImagesService)
	productDetailsHandler := httpHandler.NewProductDetailsHandler(productDetailsService)
	productAttributeChecklistHandler := httpHandler.NewProductAttributeChecklistHandler(productAttributeChecklistService)
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
	// TEMPORARY debug endpoint — remove once the category lookup it's for is
	// done.
	mercadoLibreCategoriesDebugHandler := httpHandler.NewMercadoLibreCategoriesDebugHandler(mercadoLibreCategoryPredictorService)
	meliNotificationHandler := httpHandler.NewMeliNotificationHandler(meliNotificationService)
	odooHandler := httpHandler.NewOdooHandler(odooConnectionService)
	migrationHandler := httpHandler.NewMigrationHandler(odooCategoryMigrationService, vecomSyncProductMigrationService, vecomImagesMigrationService)
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
	// TEMPORARY one-off migration handler — see
	// MercadoLibreCloseUnmappedListingsHandler. Meant to be run once, then
	// removed.
	mercadoLibreCloseUnmappedListingsHandler := httpHandler.NewMercadoLibreCloseUnmappedListingsHandler(mercadoLibreTokenService, mysqlRepos.ChannelConnectionRepository, mysqlRepos.ChannelRepository, mysqlRepos.ChannelProductMapRepository, mercadoLibreRateLimiter)
	// TEMPORARY one-off endpoint — see MercadoLibreCloseItemHandler. Remove
	// once no longer needed.
	mercadoLibreCloseItemHandler := httpHandler.NewMercadoLibreCloseItemHandler(mercadoLibreTokenService, mercadoLibreRateLimiter)
	// TEMPORARY one-off migration handler — see
	// MercadoLibreMigrateSyncItemMeliHandler. Meant to be run once, then
	// removed.
	mercadoLibreMigrateSyncItemMeliHandler := httpHandler.NewMercadoLibreMigrateSyncItemMeliHandler(db, mercadoLibreTokenService, mysqlRepos.ProductRepository, mysqlRepos.ChannelProductMapRepository, mercadoLibreRateLimiter)

	// Initialize Redis and sync service
	redisClient := redisInfra.NewClient(cfg)
	redisProductRepo := redisInfra.NewProductRepository(redisClient)
	redisInventoryRepo := redisInfra.NewInventoryRepository(redisClient)
	redisNissanRepo := redisInfra.NewNissanRepository(redisClient)
	syncService := syncApp.NewSyncService(redisProductRepo, redisInventoryRepo, redisNissanRepo, db, channelListingsService)

	// Start consumers in background so API availability is not blocked.
	// consumersDone se cierra cuando todos los supervisores de cola salieron
	// tras cancelarse bgCtx; el apagado ordenado lo espera (con plazo) para que
	// el mensaje en curso alcance a terminar.
	consumersDone := make(chan struct{})
	go func() {
		defer close(consumersDone)
		startConsumers(bgCtx, cfg, syncService)
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
	go safe.Supervise(bgCtx, "marketplace worker", marketplaceWorker.Start)

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
	go safe.Supervise(bgCtx, "token refresh scheduler", tokenRefreshScheduler.Start)

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
	go safe.Supervise(bgCtx, "compatibilities fix scheduler", compatibilitiesFixScheduler.Start)

	// Exchange rate scheduler: once a day fetches Banxico's published FIX
	// rate (SIE series SF43718) and upserts ecom_exchange_rates' USD→MXN
	// row, so EffectivePriceResolver always converts USD price lists to MXN
	// with a current rate instead of a stale hand-entered one.
	exchangeRateScheduler := schedulers.NewExchangeRateScheduler(
		sieExchangeRateUpdater,
		schedulers.ExchangeRateSchedulerConfig{
			RunAtHour:   cfg.ExchangeRateRunAtHour,
			RunAtMinute: cfg.ExchangeRateRunAtMinute,
		},
	)
	go safe.Supervise(bgCtx, "exchange rate scheduler", exchangeRateScheduler.Start)

	// Create router and start server.
	router := api.NewRouter(
		productHandler, storageDiskHandler, fileHandler, channelHandler, channelConnectionHandler,
		authHandler, authMiddleware, brandHandler, categoryHandler, currencyHandler, branchHandler,
		warehouseHandler, productStockHandler, priceListHandler, pricingFormulaHandler, productPricesHandler, exchangeRatesHandler,
		stockMovementsHandler, productImagesHandler, productDetailsHandler, productAttributeChecklistHandler, productImageImportHandler, productVideosHandler, productPartNumbersHandler,
		productDimensionsHandler, productSEOHandler, connectionCredentialsHandler, connectionSettingsHandler,
		connectionStatusHandler, channelParametersHandler, mercadoLibreHandler, meliNotificationHandler, odooHandler, sourceHandler,
		migrationHandler, equipmentTypeHandler, vehicleFitmentHandler, equipmentFitmentHandler,
		productVehicleCompatibilityHandler, productEquipmentCompatibilityHandler, channelSyncQueueHandler,
		channelListingsHandler, mercadoLibreItemLookupHandler, mercadoLibrePauseUnmappedListingsHandler,
		mercadoLibreCloseUnmappedListingsHandler, mercadoLibreCloseItemHandler,
		attributeHandler, attributeOptionHandler, productAttributeHandler, channelAttributeHandler, channelAttributeMapHandler,
		channelAttributeValueHandler,
		mercadoLibreCompatibilitiesHandler,
		mercadoLibreListingsAuditHandler,
		mercadoLibreCategoryAttributesDebugHandler,
		mercadoLibreCategoriesDebugHandler,
		mercadoLibreMigrateSyncItemMeliHandler,
	)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	tlsConfig, err := serverTLS.LoadTLSConfig(cfg.TLSKeystorePath, cfg.TLSKeystorePassword, cfg.TLSKeystoreType)
	if err != nil {
		log.Fatalf("Failed to configure TLS: %v", err)
	}

	// El servidor corre en una goroutine para que main() pueda esperar en
	// paralelo la señal de apagado (rootCtx) y disparar el cierre ordenado.
	serverErrors := make(chan error, 1)

	if tlsConfig != nil {
		server.TLSConfig = tlsConfig
		listener, listenErr := tls.Listen("tcp", server.Addr, tlsConfig)
		if listenErr != nil {
			log.Fatalf("Failed to start HTTPS server: %v", listenErr)
		}

		log.Printf("HTTPS server starting on port %s...", cfg.Port)
		go func() { serverErrors <- server.Serve(listener) }()
	} else {
		log.Printf("HTTP server starting on port %s...", cfg.Port)
		go func() { serverErrors <- server.ListenAndServe() }()
	}

	select {
	case srvErr := <-serverErrors:
		if srvErr != nil && !errors.Is(srvErr, http.ErrServerClosed) {
			log.Fatalf("server error: %v", srvErr)
		}
	case <-rootCtx.Done():
		log.Println("Señal de apagado recibida, cerrando servidor de forma ordenada...")

		// Dejar de interceptar señales: un segundo SIGINT/SIGTERM mata el
		// proceso de inmediato si el drenado se cuelga.
		stopSignals()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		// 1. Rechazar conexiones nuevas y esperar a que terminen las requests
		//    en vuelo (crear listing, actualizar precio, etc.).
		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Printf("drenado excedió %s, forzando cierre: %v", cfg.ShutdownTimeout, shutdownErr)
			_ = server.Close()
		}

		// 2. Recién ahora frenar los procesos de fondo (worker de sync a
		//    marketplaces, schedulers, consumers RabbitMQ).
		stopBackground()

		// 3. Esperar a que los consumers RabbitMQ cancelen su suscripción y
		//    terminen el mensaje en curso. Los handlers no reciben context, así
		//    que acotamos la espera para no colgar el apagado.
		select {
		case <-consumersDone:
			log.Println("Consumers RabbitMQ detenidos")
		case <-time.After(consumersShutdownGrace):
			log.Printf("Consumers RabbitMQ no terminaron en %s, saliendo igual", consumersShutdownGrace)
		}

		log.Println("Servidor detenido")
	}
}
