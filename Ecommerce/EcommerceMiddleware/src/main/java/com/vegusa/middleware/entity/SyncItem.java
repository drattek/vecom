package com.vegusa.middleware.entity;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.persistence.*;

import java.util.Date;

@Entity
@Table(name = "SyncItem")
@JsonIgnoreProperties(ignoreUnknown = true)
public class SyncItem {
    public SyncItem(){}
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    //@Column(name = "id", nullable = false)
    @Column(name = "id")
    private Long id;

    @Column(name = "id_mv", nullable = false, length = 100)
    @JsonProperty("_id")
    private String idMv;

    @Column(name = "status_mv")
    @JsonProperty("status")
    private String status;

    @Column(name = "name")
    @JsonProperty("name")
    private String name;

    @Column(name = "alias")
    @JsonProperty("alias")
    private String alias;

    @Column(name = "model")
    @JsonProperty("model")
    private String model;

    @Column(name = "description")
    @JsonProperty("description")
    private String description;

    @Column(name = "brand_id")
    @JsonProperty("BrandId")
    private String brandId;

    @Column(name = "season_id")
    @JsonProperty("SeasonId")
    private String seasonId;

    @Column(name = "product_category_id")
    @JsonProperty("ProductCategoryId")
    private String productCategoryId;

    @Column(name = "code")
    @JsonProperty("code")
    private String code;

    @Column(name = "internal_code", nullable = false)
    @JsonProperty("internalCode")
    private String internalCode;

    @Column(name = "short_description")
    @JsonProperty("shortDescription")
    private String shortDescription;

    @Column(name = "html_description")
    @JsonProperty("htmlDescription")
    private String htmlDescription;

    @Column(name = "html_short_description")
    @JsonProperty("htmlShortDescription")
    private String htmlShortDescription;

    @Column(name = "warranty_id")
    @JsonProperty("WarrantyId")
    private String warrantyId;

    @Column(name = "shipping_class_id")
    @JsonProperty("ShippingClassId")
    private String shippingClassId;

    @Column(name = "official_store_id")
    @JsonProperty("OfficialStoreId")
    private String officialStoreId;

    @Column(name = "created_by_id")
    @JsonProperty("CreatedById")
    private String createdById;

    @Column(name = "updated_by_id")
    @JsonProperty("UpdatedById")
    private String updatedById;

    @Column(name = "merchant_id")
    @JsonProperty("MerchantId")
    private String merchantId;

    @Column(name = "updated_at_mv")
    @JsonProperty("updatedAt")
    private Date updatedAt;

    @Column(name = "created_at_mv")
    @JsonProperty("createdAt")
    private Date createdAt;

    @Column(name = "product_type_Id")
    @JsonProperty("ProductTypeId")
    private String productTypeId;

    @Column(nullable = false)
    private String integration_company;

    @Column(name = "veg_sync_status")
    private String vegSyncStatus;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    private Company company;

    @Column(name = "default_version_id", length = 100)
    private String defaultVersionId;

    public String getDefaultVersionId() {
        return defaultVersionId;
    }

    public void setDefaultVersionId(String defaultVersionId) {
        this.defaultVersionId = defaultVersionId;
    }

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

    public String getIntegrationCompany(){ return this.integration_company; }
    public void setIntegrationCompany(String integration_company){ this.integration_company = integration_company; }
    public String getVegSyncStatus(){ return this.vegSyncStatus; }
    public void setVegSyncStatus(String vegSyncStatus){ this.vegSyncStatus = vegSyncStatus; }

    public Company getCompany() { return company; }
    public void setCompany(Company company) {
        this.company = company;
    }
}