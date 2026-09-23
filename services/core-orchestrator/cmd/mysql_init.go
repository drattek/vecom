package main

import (
	"database/sql"
	"log"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

type MySQLRepositories struct {
	ProductRepository                         *mysqlInfra.ProductRepository
	AuthRepository                            *mysqlInfra.AuthRepository
	StorageDiskRepository                     *mysqlInfra.StorageDiskRepository
	FilesRepository                           *mysqlInfra.FilesRepository
	ChannelRepository                         *mysqlInfra.ChannelRepository
	ChannelConnectionRepository               *mysqlInfra.ChannelConnectionRepository
	BrandsRepository                          *mysqlInfra.BrandsRepository
	CategoriesRepository                      *mysqlInfra.CategoriesRepository
	CurrenciesRepository                      *mysqlInfra.CurrenciesRepository
	SourcesRepository                         *mysqlInfra.SourcesRepository
	BranchesRepository                        *mysqlInfra.BranchesRepository
	WarehousesRepository                      *mysqlInfra.WarehousesRepository
	ProductStockRepository                    *mysqlInfra.ProductStockRepository
	PriceListRepository                       *mysqlInfra.PriceListRepository
	PricingFormulaRepository                  *mysqlInfra.PricingFormulaRepository
	ProductPricesRepository                   *mysqlInfra.ProductPricesRepository
	PriceHistoryRepository                    *mysqlInfra.PriceHistoryRepository
	ExchangeRatesRepository                   *mysqlInfra.ExchangeRatesRepository
	StockMovementsRepository                  *mysqlInfra.StockMovementsRepository
	ProductImagesRepository                   *mysqlInfra.ProductImagesRepository
	ProductVideosRepository                   *mysqlInfra.ProductVideosRepository
	ProductMediaRepository                    *mysqlInfra.ProductMediaRepository
	ProductPartNumbersRepository              *mysqlInfra.ProductPartNumbersRepository
	ProductDimensionsRepository               *mysqlInfra.ProductDimensionsRepository
	ProductSEORepository                      *mysqlInfra.ProductSEORepository
	ConnectionCredentialsRepository           *mysqlInfra.ConnectionCredentialsRepository
	ConnectionSettingsRepository              *mysqlInfra.ConnectionSettingsRepository
	ConnectionStatusRepository                *mysqlInfra.ConnectionStatusRepository
	ChannelParametersRepository               *mysqlInfra.ChannelParametersRepository
	ChannelCategoryMapRepository              *mysqlInfra.ChannelCategoryMapRepository
	EquipmentTypesRepository                  *mysqlInfra.EquipmentTypesRepository
	VehicleFitmentsRepository                 *mysqlInfra.VehicleFitmentsRepository
	EquipmentFitmentsRepository               *mysqlInfra.EquipmentFitmentsRepository
	ProductVehicleCompatibilityRepository     *mysqlInfra.ProductVehicleCompatibilityRepository
	ProductEquipmentCompatibilityRepository   *mysqlInfra.ProductEquipmentCompatibilityRepository
	ChannelSyncQueueRepository                *mysqlInfra.ChannelSyncQueueRepository
	ChannelProductMapRepository               *mysqlInfra.ChannelProductMapRepository
	PendingProductVehicleFitmentsRepository   *mysqlInfra.PendingProductVehicleFitmentsRepository
	PartNumberSupersessionsRepository         *mysqlInfra.PartNumberSupersessionsRepository
	AttributesRepository                      *mysqlInfra.AttributesRepository
	AttributeOptionsRepository                *mysqlInfra.AttributeOptionsRepository
	ProductAttributesRepository               *mysqlInfra.ProductAttributesRepository
	ChannelAttributesRepository               *mysqlInfra.ChannelAttributesRepository
	ChannelAttributeMapRepository             *mysqlInfra.ChannelAttributeMapRepository
	MeliNotificationRepository                *mysqlInfra.MeliNotificationRepository
	ProductDetailsRepository                  *mysqlInfra.ProductDetailsRepository
	ChannelProductCategorySelectionRepository *mysqlInfra.ChannelProductCategorySelectionRepository
}

func initializeMySQLRepositories(db *sql.DB) *MySQLRepositories {
	log.Println("Initializing MySQL repositories...")

	productRepository := mysqlInfra.NewProductRepository(db)
	authRepository := mysqlInfra.NewAuthRepository(db)
	storageDiskRepository := mysqlInfra.NewStorageDiskRepository(db)
	filesRepository := mysqlInfra.NewFilesRepository(db)
	channelRepository := mysqlInfra.NewChannelRepository(db)
	channelConnectionRepository := mysqlInfra.NewChannelConnectionRepository(db)
	brandsRepository := mysqlInfra.NewBrandsRepository(db)
	categoriesRepository := mysqlInfra.NewCategoriesRepository(db)
	currenciesRepository := mysqlInfra.NewCurrenciesRepository(db)
	sourcesRepository := mysqlInfra.NewSourcesRepository(db)
	branchesRepository := mysqlInfra.NewBranchesRepository(db)
	warehousesRepository := mysqlInfra.NewWarehousesRepository(db)
	productStockRepository := mysqlInfra.NewProductStockRepository(db)
	priceListRepository := mysqlInfra.NewPriceListRepository(db)
	pricingFormulaRepository := mysqlInfra.NewPricingFormulaRepository(db)
	productPricesRepository := mysqlInfra.NewProductPricesRepository(db)
	priceHistoryRepository := mysqlInfra.NewPriceHistoryRepository(db)
	exchangeRatesRepository := mysqlInfra.NewExchangeRatesRepository(db)
	stockMovementsRepository := mysqlInfra.NewStockMovementsRepository(db)
	productImagesRepository := mysqlInfra.NewProductImagesRepository(db)
	productVideosRepository := mysqlInfra.NewProductVideosRepository(db)
	productMediaRepository := mysqlInfra.NewProductMediaRepository(db)
	productPartNumbersRepository := mysqlInfra.NewProductPartNumbersRepository(db)
	productDimensionsRepository := mysqlInfra.NewProductDimensionsRepository(db)
	productSEORepository := mysqlInfra.NewProductSEORepository(db)
	connectionCredentialsRepository := mysqlInfra.NewConnectionCredentialsRepository(db)
	connectionSettingsRepository := mysqlInfra.NewConnectionSettingsRepository(db)
	connectionStatusRepository := mysqlInfra.NewConnectionStatusRepository(db)
	channelParametersRepository := mysqlInfra.NewChannelParametersRepository(db)
	channelCategoryMapRepository := mysqlInfra.NewChannelCategoryMapRepository(db)
	equipmentTypesRepository := mysqlInfra.NewEquipmentTypesRepository(db)
	vehicleFitmentsRepository := mysqlInfra.NewVehicleFitmentsRepository(db)
	equipmentFitmentsRepository := mysqlInfra.NewEquipmentFitmentsRepository(db)
	productVehicleCompatibilityRepository := mysqlInfra.NewProductVehicleCompatibilityRepository(db)
	productEquipmentCompatibilityRepository := mysqlInfra.NewProductEquipmentCompatibilityRepository(db)
	channelSyncQueueRepository := mysqlInfra.NewChannelSyncQueueRepository(db)
	channelProductMapRepository := mysqlInfra.NewChannelProductMapRepository(db)
	pendingProductVehicleFitmentsRepository := mysqlInfra.NewPendingProductVehicleFitmentsRepository(db)
	partNumberSupersessionsRepository := mysqlInfra.NewPartNumberSupersessionsRepository(db)
	attributesRepository := mysqlInfra.NewAttributesRepository(db)
	attributeOptionsRepository := mysqlInfra.NewAttributeOptionsRepository(db)
	productAttributesRepository := mysqlInfra.NewProductAttributesRepository(db)
	channelAttributesRepository := mysqlInfra.NewChannelAttributesRepository(db)
	channelAttributeMapRepository := mysqlInfra.NewChannelAttributeMapRepository(db)
	meliNotificationRepository := mysqlInfra.NewMeliNotificationRepository(db)
	productDetailsRepository := mysqlInfra.NewProductDetailsRepository(db)
	channelProductCategorySelectionRepository := mysqlInfra.NewChannelProductCategorySelectionRepository(db)

	return &MySQLRepositories{
		ProductRepository:                         productRepository,
		AuthRepository:                            authRepository,
		StorageDiskRepository:                     storageDiskRepository,
		FilesRepository:                           filesRepository,
		ChannelRepository:                         channelRepository,
		ChannelConnectionRepository:               channelConnectionRepository,
		BrandsRepository:                          brandsRepository,
		CategoriesRepository:                      categoriesRepository,
		CurrenciesRepository:                      currenciesRepository,
		SourcesRepository:                         sourcesRepository,
		BranchesRepository:                        branchesRepository,
		WarehousesRepository:                      warehousesRepository,
		ProductStockRepository:                    productStockRepository,
		PriceListRepository:                       priceListRepository,
		PricingFormulaRepository:                  pricingFormulaRepository,
		ProductPricesRepository:                   productPricesRepository,
		PriceHistoryRepository:                    priceHistoryRepository,
		ExchangeRatesRepository:                   exchangeRatesRepository,
		StockMovementsRepository:                  stockMovementsRepository,
		ProductImagesRepository:                   productImagesRepository,
		ProductVideosRepository:                   productVideosRepository,
		ProductMediaRepository:                    productMediaRepository,
		ProductPartNumbersRepository:              productPartNumbersRepository,
		ProductDimensionsRepository:               productDimensionsRepository,
		ProductSEORepository:                      productSEORepository,
		ConnectionCredentialsRepository:           connectionCredentialsRepository,
		ConnectionSettingsRepository:              connectionSettingsRepository,
		ConnectionStatusRepository:                connectionStatusRepository,
		ChannelParametersRepository:               channelParametersRepository,
		ChannelCategoryMapRepository:              channelCategoryMapRepository,
		EquipmentTypesRepository:                  equipmentTypesRepository,
		VehicleFitmentsRepository:                 vehicleFitmentsRepository,
		EquipmentFitmentsRepository:               equipmentFitmentsRepository,
		ProductVehicleCompatibilityRepository:     productVehicleCompatibilityRepository,
		ProductEquipmentCompatibilityRepository:   productEquipmentCompatibilityRepository,
		ChannelSyncQueueRepository:                channelSyncQueueRepository,
		ChannelProductMapRepository:               channelProductMapRepository,
		PendingProductVehicleFitmentsRepository:   pendingProductVehicleFitmentsRepository,
		PartNumberSupersessionsRepository:         partNumberSupersessionsRepository,
		AttributesRepository:                      attributesRepository,
		AttributeOptionsRepository:                attributeOptionsRepository,
		ProductAttributesRepository:               productAttributesRepository,
		ChannelAttributesRepository:               channelAttributesRepository,
		ChannelAttributeMapRepository:             channelAttributeMapRepository,
		MeliNotificationRepository:                meliNotificationRepository,
		ProductDetailsRepository:                  productDetailsRepository,
		ChannelProductCategorySelectionRepository: channelProductCategorySelectionRepository,
	}
}
