package com.vegusa.veg_mv_integration_midd.veg_middleware.entity;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.persistence.*;

import java.util.Date;

@Entity
@Table(name = "veg_ecomm_synchronized_products")
@JsonIgnoreProperties(ignoreUnknown = true)
public class VegMvSynchronizedProduct {

    public VegMvSynchronizedProduct(){}
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    //@Column(name = "id", nullable = false)
    @Column(name = "id")
    private Long id;

    @Column(name = "id_mv", nullable = false, length = 100)
    @JsonProperty("_id")
    private String idMv;
    @Column(name = "synchronization_Status", nullable = false, length = 50)
    @JsonProperty("synchronizationStatus")
    private String synchronizationStatus;

    @Column(name = "status_mv", length = 50)
    @JsonProperty("status")
    private String status;

    @Column(name = "name", length = 100)
    @JsonProperty("name")
    private String name;

    @Column(name = "alias", length = 100)
    @JsonProperty("alias")
    private String alias;

    @Column(name = "model", length = 50)
    @JsonProperty("model")
    private String model;

    @Column(name = "description", length = 100)
    @JsonProperty("description")
    private String description;

    @Column(name = "brand_id", length = 100)
    @JsonProperty("BrandId")
    private String brandId;

    @Column(name = "season_id", length = 100)
    @JsonProperty("SeasonId")
    private String seasonId;

    @Column(name = "product_category_id", length = 100)
    @JsonProperty("ProductCategoryId")
    private String productCategoryId;

    @Column(name = "code", length = 50)
    @JsonProperty("code")
    private String code;

    @Column(name = "internal_code", nullable = false, length = 50)
    @JsonProperty("internalCode")
    private String internalCode;

    @Column(name = "short_description", length = 100)
    @JsonProperty("shortDescription")
    private String shortDescription;

    @Column(name = "html_description", length = 250)
    @JsonProperty("htmlDescription")
    private String htmlDescription;

    @Column(name = "html_short_description", length = 250)
    @JsonProperty("htmlShortDescription")
    private String htmlShortDescription;

    @Column(name = "warranty_id", length = 100)
    @JsonProperty("WarrantyId")
    private String warrantyId;

    @Column(name = "shipping_class_id", length = 100)
    @JsonProperty("ShippingClassId")
    private String shippingClassId;

    @Column(name = "official_store_id", length = 100)
    @JsonProperty("OfficialStoreId")
    private String officialStoreId;

    @Column(name = "created_by_id", length = 100)
    @JsonProperty("CreatedById")
    private String createdById;

    @Column(name = "updated_by_id", length = 100)
    @JsonProperty("UpdatedById")
    private String updatedById;

    @Column(name = "merchant_id", length = 100)
    @JsonProperty("MerchantId")
    private String merchantId;

    @Column(name = "updated_at_mv")
    @JsonProperty("updatedAt")
    private Date updatedAt;

    @Column(name = "created_at_mv")
    @JsonProperty("createdAt")
    private Date createdAt;

    @Column(name = "product_type_Id", length = 100)
    @JsonProperty("ProductTypeId")
    private String productTypeId;
    @Column(nullable = false)
    private String integration_company;
    @Column(name = "veg_business_unit", nullable = false, length = 50)
    private String vegBusinessUnit;

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getIdMvd() {
        return idMv;
    }

    public void setIdMvd(String idMv) {
        this.idMv = idMv;
    }

    public String getSynchronizationStatus() {
        return synchronizationStatus;
    }

    public void setSynchronizationStatus(String synchronizationStatus) {
        this.synchronizationStatus = synchronizationStatus;
    }
    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getAlias() {
        return alias;
    }

    public void setAlias(String alias) {
        this.alias = alias;
    }

    public String getModel() {
        return model;
    }

    public void setModel(String model) {
        this.model = model;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getBrandId() {
        return brandId;
    }

    public void setBrandId(String BrandId) {
        this.brandId = brandId;
    }

    public String getSeasonId() {
        return seasonId;
    }

    public void setSeasonId(String SeasonId) {
        this.seasonId = seasonId;
    }

    public String getProductCategoryId() {
        return productCategoryId;
    }

    public void setProductCategoryId(String ProductCategoryId) {
        this.productCategoryId = productCategoryId;
    }

    public String getCode() {
        return code;
    }

    public void setCode(String code) {
        this.code = code;
    }

    public String getInternalCode() {
        return internalCode;
    }

    public void setInternalCode(String internalCode) {
        this.internalCode = internalCode;
    }

    public String getShortDescription() {
        return shortDescription;
    }

    public void setShortDescription(String shortDescription) {
        this.shortDescription = shortDescription;
    }

    public String getHtmlDescription() {
        return htmlDescription;
    }

    public void setHtmlDescription(String htmlDescription) {
        this.htmlDescription = htmlDescription;
    }

    public String getHtmlShortDescription() {
        return htmlShortDescription;
    }

    public void setHtmlShortDescription(String htmlShortDescription) { this.htmlShortDescription = htmlShortDescription; }

    public String getWarrantyId() {
        return warrantyId;
    }

    public void setWarrantyId(String WarrantyId) {
        this.warrantyId = warrantyId;
    }

    public String getShippingClassId() {
        return shippingClassId;
    }

    public void setShippingClassId(String ShippingClassId) {
        this.shippingClassId = shippingClassId;
    }

    public String getOfficialStoreId() {
        return officialStoreId;
    }

    public void setOfficialStoreId(String OfficialStoreId) {
        this.officialStoreId = officialStoreId;
    }

    public String getCreatedById() {
        return createdById;
    }

    public void setCreatedById(String CreatedById) {
        this.createdById = createdById;
    }

    public String getUpdatedById() {
        return updatedById;
    }

    public void setUpdatedById(String UpdatedById) {
        this.updatedById = UpdatedById;
    }

    public String getMerchantId() {
        return merchantId;
    }

    public void setMerchantId(String MerchantId) {
        this.merchantId = merchantId;
    }

    public Date getUpdatedAtMv() {
        return updatedAt;
    }

    public void setUpdatedAtMv(Date updatedAt) {
        this.updatedAt = updatedAt;
    }

    public Date getCreatedAtMv() {
        return createdAt;
    }

    public void setCreatedAtMv(Date createdAt) {
        this.createdAt = createdAt;
    }

    public String getProductTypeId() {
        return productTypeId;
    }

    public void setProductTypeId(String ProductTypeId) {
        this.productTypeId = productTypeId;
    }

    public String getVegBusinessUnit() { return vegBusinessUnit; }
    public void setVegBusinessUnit(String vegBusinessUnit) { this.vegBusinessUnit = vegBusinessUnit; }
    public String getIntegrationCompany(){ return this.integration_company; }
    public void setIntegrationCompany(String integration_company){ this.integration_company = integration_company; }


}