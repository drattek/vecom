package com.vegusa.veg_mv_integration_midd.msb.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import org.hibernate.annotations.Immutable;
import org.hibernate.annotations.Nationalized;

import java.math.BigDecimal;
import java.time.Instant;

/**
 * Mapping for DB view
 */
@Entity
@Immutable
public class InventTable {
    @Id
    @Column(name = "RECID", nullable = false)
    private Long recid;

    @Column(name = "\"$FileName\"", length = 100)
    private String $FileName;

    @Column(name = "_SysRowId", nullable = false)
    private Long sysrowid;

    @Nationalized
    @Column(name = "LSN", nullable = false, length = 60)
    private String lsn;

    @Column(name = "LastProcessedChange_DateTime", nullable = false)
    private Instant lastprocessedchangeDatetime;

    @Column(name = "DataLakeModified_DateTime", nullable = false)
    private Instant datalakemodifiedDatetime;

    @Nationalized
    @Column(name = "ItemId", nullable = false, length = 20)
    private String itemId;

    @Column(name = "ABCContributionMargin")
    private Integer aBCContributionMargin;

    @Column(name = "ABCRevenue")
    private Integer aBCRevenue;

    @Column(name = "ABCTieUp")
    private Integer aBCTieUp;

    @Column(name = "ABCValue")
    private Integer aBCValue;

    @Nationalized
    @Column(name = "AlcoholManufacturerId_RU", length = 20)
    private String alcoholmanufactureridRu;

    @Nationalized
    @Column(name = "AlcoholProductionTypeId_RU", length = 10)
    private String alcoholproductiontypeidRu;

    @Column(name = "AlcoholStrength_RU", precision = 32, scale = 16)
    private BigDecimal alcoholstrengthRu;

    @Nationalized
    @Column(name = "AltConfigId", length = 50)
    private String altConfigId;

    @Nationalized
    @Column(name = "AltInventColorId", length = 60)
    private String altInventColorId;

    @Nationalized
    @Column(name = "AltInventSizeId", length = 60)
    private String altInventSizeId;

    @Nationalized
    @Column(name = "AltInventStyleId", length = 60)
    private String altInventStyleId;

    @Nationalized
    @Column(name = "AltInventVersionId", length = 10)
    private String altInventVersionId;

    @Nationalized
    @Column(name = "AltItemId", length = 20)
    private String altItemId;

    @Column(name = "ApproxTaxValue_BR", precision = 32, scale = 16)
    private BigDecimal approxtaxvalueBr;

    @Nationalized
    @Column(name = "AssetGroupId_RU", length = 10)
    private String assetgroupidRu;

    @Nationalized
    @Column(name = "AssetId_RU", length = 20)
    private String assetidRu;

    @Column(name = "AutoReportFinished")
    private Integer autoReportFinished;

    @Column(name = "BatchMergeDateCalculationMethod")
    private Integer batchMergeDateCalculationMethod;

    @Nationalized
    @Column(name = "BatchNumGroupId", length = 10)
    private String batchNumGroupId;

    @Nationalized
    @Column(name = "BOMCalcGroupId", length = 10)
    private String bOMCalcGroupId;

    @Column(name = "BOMLevel")
    private Integer bOMLevel;

    @Column(name = "BOMManualReceipt")
    private Integer bOMManualReceipt;

    @Nationalized
    @Column(name = "BOMUnitId", length = 10)
    private String bOMUnitId;

    @Nationalized
    @Column(name = "BrandCodeId_MX", length = 30)
    private String brandcodeidMx;

    @Nationalized
    @Column(name = "CommissionGroupId", length = 10)
    private String commissionGroupId;

    @Nationalized
    @Column(name = "CostGroupId", length = 10)
    private String costGroupId;

    @Column(name = "CostModel")
    private Integer costModel;

    @Column(name = "CustomsExportTariffCodeTable_IN")
    private Long customsexporttariffcodetableIn;

    @Column(name = "CustomsImportTariffCodeTable_IN")
    private Long customsimporttariffcodetableIn;

    @Column(name = "DefaultDimension")
    private Long defaultDimension;

    @Column(name = "Density", precision = 32, scale = 16)
    private BigDecimal density;

    @Column(name = "Depth", precision = 32, scale = 16)
    private BigDecimal depth;

    @Nationalized
    @Column(name = "ExceptionCode_BR", length = 10)
    private String exceptioncodeBr;

    @Column(name = "ExciseTariffCodes_IN")
    private Long excisetariffcodesIn;

    @Column(name = "EximProductGroupTable_IN")
    private Long eximproductgrouptableIn;

    @Column(name = "FiscalLIFOAvoidCalc")
    private Integer fiscalLIFOAvoidCalc;

    @Column(name = "FiscalLIFONormalValue", precision = 32, scale = 16)
    private BigDecimal fiscalLIFONormalValue;

    @Column(name = "FiscalLIFONormalValueCalc")
    private Integer fiscalLIFONormalValueCalc;

    @Column(name = "ForecastDMPInclude")
    private Integer forecastDMPInclude;

    @Column(name = "grossDepth", precision = 32, scale = 16)
    private BigDecimal grossDepth;

    @Column(name = "grossHeight", precision = 32, scale = 16)
    private BigDecimal grossHeight;

    @Column(name = "grossWidth", precision = 32, scale = 16)
    private BigDecimal grossWidth;

    @Column(name = "Height", precision = 32, scale = 16)
    private BigDecimal height;

    @Column(name = "ICMSOnService_BR")
    private Integer icmsonserviceBr;

    @Column(name = "IntrastatCommodity")
    private Long intrastatCommodity;

    @Column(name = "IntrastatExclude")
    private Integer intrastatExclude;

    @Nationalized
    @Column(name = "IntrastatProcId_CZ", length = 10)
    private String intrastatprocidCz;

    @Column(name = "InventFiscalLIFOGroup")
    private Long inventFiscalLIFOGroup;

    @Nationalized
    @Column(name = "InventProductType_BR", length = 10)
    private String inventproducttypeBr;

    @Nationalized
    @Column(name = "ItemBuyerGroupId", length = 10)
    private String itemBuyerGroupId;

    @Column(name = "ItemDimCostPrice")
    private Integer itemDimCostPrice;

    @Nationalized
    @Column(name = "ItemPriceToleranceGroupId", length = 10)
    private String itemPriceToleranceGroupId;

    @Column(name = "ItemType")
    private Integer itemType;

    @Nationalized
    @Column(name = "MarkupCode_RU", length = 10)
    private String markupcodeRu;

    @Column(name = "MatchingPolicy")
    private Integer matchingPolicy;

    @Column(name = "MINIMUMPALLETQUANTITY", nullable = false, precision = 32, scale = 16)
    private BigDecimal minimumpalletquantity;

    @Nationalized
    @Column(name = "NameAlias", length = 20)
    private String nameAlias;

    @Column(name = "NetWeight", precision = 32, scale = 16)
    private BigDecimal netWeight;

    @Column(name = "NGPCodesTable_FR")
    private Long ngpcodestableFr;

    @Nationalized
    @Column(name = "NRTaxGroup_LV", length = 10)
    private String nrtaxgroupLv;

    @Nationalized
    @Column(name = "OrigCountryRegionId", length = 10)
    private String origCountryRegionId;

    @Nationalized
    @Column(name = "OrigCountyId", length = 30)
    private String origCountyId;

    @Nationalized
    @Column(name = "OrigStateId")
    private String origStateId;

    @Nationalized
    @Column(name = "PackagingGroupId", length = 10)
    private String packagingGroupId;

    @Nationalized
    @Column(name = "Packing_RU", length = 20)
    private String packingRu;

    @Nationalized
    @Column(name = "PDSBaseAttributeId", length = 20)
    private String pDSBaseAttributeId;

    @Column(name = "PdsBestBefore")
    private Integer pdsBestBefore;

    @Column(name = "PDSCWWMSMINIMUMPALLETQTY", nullable = false, precision = 32, scale = 16)
    private BigDecimal pdscwwmsminimumpalletqty;

    @Column(name = "PDSCWWMSQTYPERLAYER", nullable = false, precision = 32, scale = 16)
    private BigDecimal pdscwwmsqtyperlayer;

    @Column(name = "PDSCWWMSSTANDARDPALLETQTY", nullable = false, precision = 32, scale = 16)
    private BigDecimal pdscwwmsstandardpalletqty;

    @Nationalized
    @Column(name = "PdsFreightAllocationGroupId", length = 10)
    private String pdsFreightAllocationGroupId;

    @Nationalized
    @Column(name = "PdsItemRebateGroupId", length = 10)
    private String pdsItemRebateGroupId;

    @Column(name = "PDSPotencyAttribRecording")
    private Integer pDSPotencyAttribRecording;

    @Column(name = "PdsShelfAdvice")
    private Integer pdsShelfAdvice;

    @Column(name = "PdsShelfLife")
    private Integer pdsShelfLife;

    @Column(name = "PDSTargetFactor", precision = 32, scale = 16)
    private BigDecimal pDSTargetFactor;

    @Column(name = "PdsVendorCheckItem")
    private Integer pdsVendorCheckItem;

    @Column(name = "Phantom")
    private Integer phantom;

    @Nationalized
    @Column(name = "PKWiUCode_PL", length = 20)
    private String pkwiucodePl;

    @Nationalized
    @Column(name = "PmfPlanningItemId", length = 20)
    private String pmfPlanningItemId;

    @Column(name = "PmfProductType")
    private Integer pmfProductType;

    @Column(name = "PmfYieldPct", precision = 32, scale = 16)
    private BigDecimal pmfYieldPct;

    @Nationalized
    @Column(name = "PrimaryVendorId", length = 20)
    private String primaryVendorId;

    @Column(name = "ProdFlushingPrincip")
    private Integer prodFlushingPrincip;

    @Nationalized
    @Column(name = "ProdGroupId", length = 10)
    private String prodGroupId;

    @Nationalized
    @Column(name = "ProdPoolId", length = 10)
    private String prodPoolId;

    @Column(name = "Product", nullable = false)
    private Long product;

    @Nationalized
    @Column(name = "projCategoryId", length = 30)
    private String projCategoryId;

    @Nationalized
    @Column(name = "PropertyId", length = 10)
    private String propertyId;

    @Column(name = "PurchModel")
    private Integer purchModel;

    @Column(name = "QTYPERLAYER", nullable = false, precision = 32, scale = 16)
    private BigDecimal qtyperlayer;

    @Nationalized
    @Column(name = "ReqGroupId", length = 10)
    private String reqGroupId;

    @Nationalized
    @Column(name = "SADRateCode_PL", length = 10)
    private String sadratecodePl;

    @Column(name = "SalesContributionRatio", precision = 32, scale = 16)
    private BigDecimal salesContributionRatio;

    @Column(name = "SalesModel")
    private Integer salesModel;

    @Column(name = "SalesPercentMarkup", precision = 32, scale = 16)
    private BigDecimal salesPercentMarkup;

    @Column(name = "SalesPriceModelBasic")
    private Integer salesPriceModelBasic;

    @Column(name = "ScrapConst", precision = 32, scale = 16)
    private BigDecimal scrapConst;

    @Column(name = "ScrapVar", precision = 32, scale = 16)
    private BigDecimal scrapVar;

    @Nationalized
    @Column(name = "SerialNumGroupId", length = 10)
    private String serialNumGroupId;

    @Column(name = "ServiceCodeTable_IN")
    private Long servicecodetableIn;

    @Column(name = "SkipIntraCompanySync_RU")
    private Integer skipintracompanysyncRu;

    @Column(name = "sortCode")
    private Integer sortCode;

    @Nationalized
    @Column(name = "StandardConfigId", length = 50)
    private String standardConfigId;

    @Nationalized
    @Column(name = "StandardInventColorId", length = 60)
    private String standardInventColorId;

    @Nationalized
    @Column(name = "StandardInventSizeId", length = 60)
    private String standardInventSizeId;

    @Nationalized
    @Column(name = "StandardInventStyleId", length = 60)
    private String standardInventStyleId;

    @Nationalized
    @Column(name = "StandardInventVersionId", length = 10)
    private String standardInventVersionId;

    @Column(name = "STANDARDPALLETQUANTITY", nullable = false, precision = 32, scale = 16)
    private BigDecimal standardpalletquantity;

    @Column(name = "StatisticsFactor", precision = 32, scale = 16)
    private BigDecimal statisticsFactor;

    @Column(name = "TaraWeight", precision = 32, scale = 16)
    private BigDecimal taraWeight;

    @Column(name = "TaxationOrigin_BR")
    private Integer taxationoriginBr;

    @Nationalized
    @Column(name = "TaxFiscalClassification_BR", length = 10)
    private String taxfiscalclassificationBr;

    @Column(name = "TaxPackagingQty", precision = 32, scale = 16)
    private BigDecimal taxPackagingQty;

    @Nationalized
    @Column(name = "TaxServiceCode_BR", length = 10)
    private String taxservicecodeBr;

    @Column(name = "UnitVolume", precision = 32, scale = 16)
    private BigDecimal unitVolume;

    @Column(name = "UseAltItemId")
    private Integer useAltItemId;

    @Column(name = "Width", precision = 32, scale = 16)
    private BigDecimal width;

    @Column(name = "WMSArrivalHandlingTime")
    private Integer wMSArrivalHandlingTime;

    @Nationalized
    @Column(name = "WMSPALLETTYPEID", nullable = false, length = 1000)
    private String wmspallettypeid;

    @Column(name = "WMSPICKINGQTYTIME", nullable = false)
    private Integer wmspickingqtytime;

    @Column(name = "DSA_IN")
    private Integer dsaIn;

    @Column(name = "ExciseRecordType_IN")
    private Integer exciserecordtypeIn;

    @Nationalized
    @Column(name = "SATCodeId_MX", length = 10)
    private String satcodeidMx;

    @Nationalized
    @Column(name = "SATTariffFraction_MX", length = 10)
    private String sattarifffractionMx;

    @Column(name = "HSNCodeTable_IN")
    private Long hsncodetableIn;

    @Column(name = "ServiceAccountingCodeTable_IN")
    private Long serviceaccountingcodetableIn;

    @Column(name = "Exempt_IN")
    private Integer exemptIn;

    @Nationalized
    @Column(name = "ProductLifecycleStateId", length = 60)
    private String productLifecycleStateId;

    @Column(name = "ScaleIndicator_BR")
    private Integer scaleindicatorBr;

    @Nationalized
    @Column(name = "CNPJ_BR", length = 20)
    private String cnpjBr;

    @Column(name = "NonGST_IN")
    private Integer nongstIn;

    @Column(name = "TaxRateType")
    private Long taxRateType;

    @Column(name = "IntrastatChargePerKg", precision = 32, scale = 16)
    private BigDecimal intrastatChargePerKg;

    @Column(name = "HMIMIndicator")
    private Integer hMIMIndicator;

    @Column(name = "COODualUseProduct")
    private Integer cOODualUseProduct;

    @Nationalized
    @Column(name = "COODualUseCode", length = 10)
    private String cOODualUseCode;

    @Column(name = "CostBOMLevel")
    private Integer costBOMLevel;

    @Nationalized
    @Column(name = "FreeNotesGroup_IT", length = 10)
    private String freenotesgroupIt;

    @Column(name = "DisplayHazard_MX")
    private Integer displayhazardMx;

    @Nationalized
    @Column(name = "ITMArrivalGroupId", length = 10)
    private String iTMArrivalGroupId;

    @Nationalized
    @Column(name = "ITMCommodityCodeId", length = 10)
    private String iTMCommodityCodeId;

    @Nationalized
    @Column(name = "ITMCustomsDescId", length = 60)
    private String iTMCustomsDescId;

    @Nationalized
    @Column(name = "ITMOverUnderToleranceGroupId", length = 10)
    private String iTMOverUnderToleranceGroupId;

    @Nationalized
    @Column(name = "ITMCostTypeGroupId", length = 20)
    private String iTMCostTypeGroupId;

    @Nationalized
    @Column(name = "ITMCostTransferGroupId", length = 10)
    private String iTMCostTransferGroupId;

    @Nationalized
    @Column(name = "RevRecDefaultRevenueRecognitionSchedule", length = 10)
    private String revRecDefaultRevenueRecognitionSchedule;

    @Column(name = "RevRecExcludeFromCarveOut")
    private Integer revRecExcludeFromCarveOut;

    @Column(name = "RevRecMedianPrice")
    private Integer revRecMedianPrice;

    @Column(name = "RevRecMedianPriceMaximumTolerance", precision = 32, scale = 16)
    private BigDecimal revRecMedianPriceMaximumTolerance;

    @Column(name = "RevRecMedianPriceMinimumTolerance", precision = 32, scale = 16)
    private BigDecimal revRecMedianPriceMinimumTolerance;

    @Column(name = "RevRecRevenueRecognitionEnabled")
    private Integer revRecRevenueRecognitionEnabled;

    @Column(name = "RevRecRevenueType")
    private Integer revRecRevenueType;

    @Column(name = "RevRecBundle")
    private Integer revRecBundle;

    @Nationalized
    @Column(name = "DataAreaId", nullable = false, length = 4)
    private String dataAreaId;

    @Column(name = "PARTITION", nullable = false)
    private Long partition;

    @Column(name = "RECVERSION", nullable = false)
    private Integer recversion;

    @Column(name = "MODIFIEDDATETIME", nullable = false)
    private Instant modifieddatetime;

    @Nationalized
    @Column(name = "MODIFIEDBY", nullable = false, length = 20)
    private String modifiedby;

    @Column(name = "CREATEDDATETIME", nullable = false)
    private Instant createddatetime;

    @Nationalized
    @Column(name = "CREATEDBY", nullable = false, length = 20)
    private String createdby;

    @Nationalized
    @Column(name = "Grupo_Custom", length = 10)
    private String grupoCustom;

    @Nationalized
    @Column(name = "SYSSHARINGDATAAREAID", nullable = false, length = 1000)
    private String syssharingdataareaid;

    @Column(name = "BomWHSReleasePolicy")
    private Integer bomWHSReleasePolicy;

    @Column(name = "Bundle")
    private Integer bundle;

    public Long getRecid() {
        return recid;
    }

    public String get$FileName() {
        return $FileName;
    }

    public Long getSysrowid() {
        return sysrowid;
    }

    public String getLsn() {
        return lsn;
    }

    public Instant getLastprocessedchangeDatetime() {
        return lastprocessedchangeDatetime;
    }

    public Instant getDatalakemodifiedDatetime() {
        return datalakemodifiedDatetime;
    }

    public String getItemId() {
        return itemId;
    }

    public Integer getABCContributionMargin() {
        return aBCContributionMargin;
    }

    public Integer getABCRevenue() {
        return aBCRevenue;
    }

    public Integer getABCTieUp() {
        return aBCTieUp;
    }

    public Integer getABCValue() {
        return aBCValue;
    }

    public String getAlcoholmanufactureridRu() {
        return alcoholmanufactureridRu;
    }

    public String getAlcoholproductiontypeidRu() {
        return alcoholproductiontypeidRu;
    }

    public BigDecimal getAlcoholstrengthRu() {
        return alcoholstrengthRu;
    }

    public String getAltConfigId() {
        return altConfigId;
    }

    public String getAltInventColorId() {
        return altInventColorId;
    }

    public String getAltInventSizeId() {
        return altInventSizeId;
    }

    public String getAltInventStyleId() {
        return altInventStyleId;
    }

    public String getAltInventVersionId() {
        return altInventVersionId;
    }

    public String getAltItemId() {
        return altItemId;
    }

    public BigDecimal getApproxtaxvalueBr() {
        return approxtaxvalueBr;
    }

    public String getAssetgroupidRu() {
        return assetgroupidRu;
    }

    public String getAssetidRu() {
        return assetidRu;
    }

    public Integer getAutoReportFinished() {
        return autoReportFinished;
    }

    public Integer getBatchMergeDateCalculationMethod() {
        return batchMergeDateCalculationMethod;
    }

    public String getBatchNumGroupId() {
        return batchNumGroupId;
    }

    public String getBOMCalcGroupId() {
        return bOMCalcGroupId;
    }

    public Integer getBOMLevel() {
        return bOMLevel;
    }

    public Integer getBOMManualReceipt() {
        return bOMManualReceipt;
    }

    public String getBOMUnitId() {
        return bOMUnitId;
    }

    public String getBrandcodeidMx() {
        return brandcodeidMx;
    }

    public String getCommissionGroupId() {
        return commissionGroupId;
    }

    public String getCostGroupId() {
        return costGroupId;
    }

    public Integer getCostModel() {
        return costModel;
    }

    public Long getCustomsexporttariffcodetableIn() {
        return customsexporttariffcodetableIn;
    }

    public Long getCustomsimporttariffcodetableIn() {
        return customsimporttariffcodetableIn;
    }

    public Long getDefaultDimension() {
        return defaultDimension;
    }

    public BigDecimal getDensity() {
        return density;
    }

    public BigDecimal getDepth() {
        return depth;
    }

    public String getExceptioncodeBr() {
        return exceptioncodeBr;
    }

    public Long getExcisetariffcodesIn() {
        return excisetariffcodesIn;
    }

    public Long getEximproductgrouptableIn() {
        return eximproductgrouptableIn;
    }

    public Integer getFiscalLIFOAvoidCalc() {
        return fiscalLIFOAvoidCalc;
    }

    public BigDecimal getFiscalLIFONormalValue() {
        return fiscalLIFONormalValue;
    }

    public Integer getFiscalLIFONormalValueCalc() {
        return fiscalLIFONormalValueCalc;
    }

    public Integer getForecastDMPInclude() {
        return forecastDMPInclude;
    }

    public BigDecimal getGrossDepth() {
        return grossDepth;
    }

    public BigDecimal getGrossHeight() {
        return grossHeight;
    }

    public BigDecimal getGrossWidth() {
        return grossWidth;
    }

    public BigDecimal getHeight() {
        return height;
    }

    public Integer getIcmsonserviceBr() {
        return icmsonserviceBr;
    }

    public Long getIntrastatCommodity() {
        return intrastatCommodity;
    }

    public Integer getIntrastatExclude() {
        return intrastatExclude;
    }

    public String getIntrastatprocidCz() {
        return intrastatprocidCz;
    }

    public Long getInventFiscalLIFOGroup() {
        return inventFiscalLIFOGroup;
    }

    public String getInventproducttypeBr() {
        return inventproducttypeBr;
    }

    public String getItemBuyerGroupId() {
        return itemBuyerGroupId;
    }

    public Integer getItemDimCostPrice() {
        return itemDimCostPrice;
    }

    public String getItemPriceToleranceGroupId() {
        return itemPriceToleranceGroupId;
    }

    public Integer getItemType() {
        return itemType;
    }

    public String getMarkupcodeRu() {
        return markupcodeRu;
    }

    public Integer getMatchingPolicy() {
        return matchingPolicy;
    }

    public BigDecimal getMinimumpalletquantity() {
        return minimumpalletquantity;
    }

    public String getNameAlias() {
        return nameAlias;
    }

    public BigDecimal getNetWeight() {
        return netWeight;
    }

    public Long getNgpcodestableFr() {
        return ngpcodestableFr;
    }

    public String getNrtaxgroupLv() {
        return nrtaxgroupLv;
    }

    public String getOrigCountryRegionId() {
        return origCountryRegionId;
    }

    public String getOrigCountyId() {
        return origCountyId;
    }

    public String getOrigStateId() {
        return origStateId;
    }

    public String getPackagingGroupId() {
        return packagingGroupId;
    }

    public String getPackingRu() {
        return packingRu;
    }

    public String getPDSBaseAttributeId() {
        return pDSBaseAttributeId;
    }

    public Integer getPdsBestBefore() {
        return pdsBestBefore;
    }

    public BigDecimal getPdscwwmsminimumpalletqty() {
        return pdscwwmsminimumpalletqty;
    }

    public BigDecimal getPdscwwmsqtyperlayer() {
        return pdscwwmsqtyperlayer;
    }

    public BigDecimal getPdscwwmsstandardpalletqty() {
        return pdscwwmsstandardpalletqty;
    }

    public String getPdsFreightAllocationGroupId() {
        return pdsFreightAllocationGroupId;
    }

    public String getPdsItemRebateGroupId() {
        return pdsItemRebateGroupId;
    }

    public Integer getPDSPotencyAttribRecording() {
        return pDSPotencyAttribRecording;
    }

    public Integer getPdsShelfAdvice() {
        return pdsShelfAdvice;
    }

    public Integer getPdsShelfLife() {
        return pdsShelfLife;
    }

    public BigDecimal getPDSTargetFactor() {
        return pDSTargetFactor;
    }

    public Integer getPdsVendorCheckItem() {
        return pdsVendorCheckItem;
    }

    public Integer getPhantom() {
        return phantom;
    }

    public String getPkwiucodePl() {
        return pkwiucodePl;
    }

    public String getPmfPlanningItemId() {
        return pmfPlanningItemId;
    }

    public Integer getPmfProductType() {
        return pmfProductType;
    }

    public BigDecimal getPmfYieldPct() {
        return pmfYieldPct;
    }

    public String getPrimaryVendorId() {
        return primaryVendorId;
    }

    public Integer getProdFlushingPrincip() {
        return prodFlushingPrincip;
    }

    public String getProdGroupId() {
        return prodGroupId;
    }

    public String getProdPoolId() {
        return prodPoolId;
    }

    public Long getProduct() {
        return product;
    }

    public String getProjCategoryId() {
        return projCategoryId;
    }

    public String getPropertyId() {
        return propertyId;
    }

    public Integer getPurchModel() {
        return purchModel;
    }

    public BigDecimal getQtyperlayer() {
        return qtyperlayer;
    }

    public String getReqGroupId() {
        return reqGroupId;
    }

    public String getSadratecodePl() {
        return sadratecodePl;
    }

    public BigDecimal getSalesContributionRatio() {
        return salesContributionRatio;
    }

    public Integer getSalesModel() {
        return salesModel;
    }

    public BigDecimal getSalesPercentMarkup() {
        return salesPercentMarkup;
    }

    public Integer getSalesPriceModelBasic() {
        return salesPriceModelBasic;
    }

    public BigDecimal getScrapConst() {
        return scrapConst;
    }

    public BigDecimal getScrapVar() {
        return scrapVar;
    }

    public String getSerialNumGroupId() {
        return serialNumGroupId;
    }

    public Long getServicecodetableIn() {
        return servicecodetableIn;
    }

    public Integer getSkipintracompanysyncRu() {
        return skipintracompanysyncRu;
    }

    public Integer getSortCode() {
        return sortCode;
    }

    public String getStandardConfigId() {
        return standardConfigId;
    }

    public String getStandardInventColorId() {
        return standardInventColorId;
    }

    public String getStandardInventSizeId() {
        return standardInventSizeId;
    }

    public String getStandardInventStyleId() {
        return standardInventStyleId;
    }

    public String getStandardInventVersionId() {
        return standardInventVersionId;
    }

    public BigDecimal getStandardpalletquantity() {
        return standardpalletquantity;
    }

    public BigDecimal getStatisticsFactor() {
        return statisticsFactor;
    }

    public BigDecimal getTaraWeight() {
        return taraWeight;
    }

    public Integer getTaxationoriginBr() {
        return taxationoriginBr;
    }

    public String getTaxfiscalclassificationBr() {
        return taxfiscalclassificationBr;
    }

    public BigDecimal getTaxPackagingQty() {
        return taxPackagingQty;
    }

    public String getTaxservicecodeBr() {
        return taxservicecodeBr;
    }

    public BigDecimal getUnitVolume() {
        return unitVolume;
    }

    public Integer getUseAltItemId() {
        return useAltItemId;
    }

    public BigDecimal getWidth() {
        return width;
    }

    public Integer getWMSArrivalHandlingTime() {
        return wMSArrivalHandlingTime;
    }

    public String getWmspallettypeid() {
        return wmspallettypeid;
    }

    public Integer getWmspickingqtytime() {
        return wmspickingqtytime;
    }

    public Integer getDsaIn() {
        return dsaIn;
    }

    public Integer getExciserecordtypeIn() {
        return exciserecordtypeIn;
    }

    public String getSatcodeidMx() {
        return satcodeidMx;
    }

    public String getSattarifffractionMx() {
        return sattarifffractionMx;
    }

    public Long getHsncodetableIn() {
        return hsncodetableIn;
    }

    public Long getServiceaccountingcodetableIn() {
        return serviceaccountingcodetableIn;
    }

    public Integer getExemptIn() {
        return exemptIn;
    }

    public String getProductLifecycleStateId() {
        return productLifecycleStateId;
    }

    public Integer getScaleindicatorBr() {
        return scaleindicatorBr;
    }

    public String getCnpjBr() {
        return cnpjBr;
    }

    public Integer getNongstIn() {
        return nongstIn;
    }

    public Long getTaxRateType() {
        return taxRateType;
    }

    public BigDecimal getIntrastatChargePerKg() {
        return intrastatChargePerKg;
    }

    public Integer getHMIMIndicator() {
        return hMIMIndicator;
    }

    public Integer getCOODualUseProduct() {
        return cOODualUseProduct;
    }

    public String getCOODualUseCode() {
        return cOODualUseCode;
    }

    public Integer getCostBOMLevel() {
        return costBOMLevel;
    }

    public String getFreenotesgroupIt() {
        return freenotesgroupIt;
    }

    public Integer getDisplayhazardMx() {
        return displayhazardMx;
    }

    public String getITMArrivalGroupId() {
        return iTMArrivalGroupId;
    }

    public String getITMCommodityCodeId() {
        return iTMCommodityCodeId;
    }

    public String getITMCustomsDescId() {
        return iTMCustomsDescId;
    }

    public String getITMOverUnderToleranceGroupId() {
        return iTMOverUnderToleranceGroupId;
    }

    public String getITMCostTypeGroupId() {
        return iTMCostTypeGroupId;
    }

    public String getITMCostTransferGroupId() {
        return iTMCostTransferGroupId;
    }

    public String getRevRecDefaultRevenueRecognitionSchedule() {
        return revRecDefaultRevenueRecognitionSchedule;
    }

    public Integer getRevRecExcludeFromCarveOut() {
        return revRecExcludeFromCarveOut;
    }

    public Integer getRevRecMedianPrice() {
        return revRecMedianPrice;
    }

    public BigDecimal getRevRecMedianPriceMaximumTolerance() {
        return revRecMedianPriceMaximumTolerance;
    }

    public BigDecimal getRevRecMedianPriceMinimumTolerance() {
        return revRecMedianPriceMinimumTolerance;
    }

    public Integer getRevRecRevenueRecognitionEnabled() {
        return revRecRevenueRecognitionEnabled;
    }

    public Integer getRevRecRevenueType() {
        return revRecRevenueType;
    }

    public Integer getRevRecBundle() {
        return revRecBundle;
    }

    public String getDataAreaId() {
        return dataAreaId;
    }

    public Long getPartition() {
        return partition;
    }

    public Integer getRecversion() {
        return recversion;
    }

    public Instant getModifieddatetime() {
        return modifieddatetime;
    }

    public String getModifiedby() {
        return modifiedby;
    }

    public Instant getCreateddatetime() {
        return createddatetime;
    }

    public String getCreatedby() {
        return createdby;
    }

    public String getGrupoCustom() {
        return grupoCustom;
    }

    public String getSyssharingdataareaid() {
        return syssharingdataareaid;
    }

    public Integer getBomWHSReleasePolicy() {
        return bomWHSReleasePolicy;
    }

    public Integer getBundle() {
        return bundle;
    }

    protected InventTable() {
    }
}