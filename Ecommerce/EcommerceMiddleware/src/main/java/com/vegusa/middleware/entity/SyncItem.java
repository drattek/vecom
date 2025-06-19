package com.vegusa.middleware.entity;

import com.fasterxml.jackson.annotation.JsonIgnore;
import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonInclude;
import jakarta.persistence.*;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.Date;

@Entity
@Table(name = "SyncItem")
@JsonIgnoreProperties(ignoreUnknown = true)
@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class SyncItem {
    public SyncItem(){}
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId")
    @JsonIgnore
    private Long recId;

    @Column(name = "ResponseId", nullable = false, length = 100)
    @JsonProperty("_id")
    private String responseId;

    @Column(name = "ResponseStatus")
    @JsonProperty("status")
    private String responseStatus;

    @Column(name = "Name")
    @JsonProperty("name")
    private String name;

    @Column(name = "Alias")
    @JsonProperty("alias")
    private String alias;

    @Column(name = "Model")
    @JsonProperty("model")
    private String model;

    @Column(name = "Description")
    @JsonProperty("description")
    private String description;

    @Column(name = "BrandId")
    @JsonProperty("BrandId")
    private String brandId;

    @Column(name = "SeasonId")
    @JsonProperty("SeasonId")
    private String seasonId;

    @Column(name = "ProductCategoryId")
    @JsonProperty("ProductCategoryId")
    private String productCategoryId;

    @Column(name = "Code")
    @JsonProperty("code")
    private String code;

    @Column(name = "InternalCode", nullable = false)
    @JsonProperty("internalCode")
    private String internalCode;

    @Column(name = "ShortDescription")
    @JsonProperty("shortDescription")
    private String shortDescription;

    @Column(name = "HtmlDescription")
    @JsonProperty("htmlDescription")
    private String htmlDescription;

    @Column(name = "HtmlShortDescription")
    @JsonProperty("htmlShortDescription")
    private String htmlShortDescription;

    @Column(name = "WarrantyId")
    @JsonProperty("WarrantyId")
    private String warrantyId;

    @Column(name = "ShippingClassId")
    @JsonProperty("ShippingClassId")
    private String shippingClassId;

    @Column(name = "OfficialStoreId")
    @JsonProperty("OfficialStoreId")
    private String officialStoreId;

    @Column(name = "CreatedById")
    @JsonProperty("CreatedById")
    private String createdById;

    @Column(name = "UpdatedById")
    @JsonProperty("UpdatedById")
    private String updatedById;

    @Column(name = "MerchantId")
    @JsonProperty("MerchantId")
    private String merchantId;

    @Column(name = "UpdatedAt")
    @JsonProperty("updatedAt")
    private Date updatedAt;

    @Column(name = "CreatedAt")
    @JsonProperty("createdAt")
    private Date createdAt;

    @Column(name = "ProductTypeId")
    @JsonProperty("ProductTypeId")
    private String productTypeId;

    @Column(name="IntegrationCompany", nullable = false)
    @JsonIgnore
    private String integrationCompany;

    @Column(name = "DefaultVersionId")
    @JsonIgnore
    private String defaultVersionId;

    @Column(name = "VegSyncStatus")
    @JsonIgnore
    private String vegSyncStatus;

    /*
    @OneToOne
    @JsonProperty("ProductVersions")
    private SyncProductVersion[] syncProductVersions;
    */

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    @JsonIgnore
    private Company company;

    public Long getRecId() {
        return recId;
    }

    public void setRecId(Long recId) {
        this.recId = recId;
    }

    public String getResponseId() {
        return responseId;
    }

    public void setResponseId(String responseId) {
        this.responseId = responseId;
    }

    public String getResponseStatus() {
        return responseStatus;
    }

    public void setResponseStatus(String responseStatus) {
        this.responseStatus = responseStatus;
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

    public void setBrandId(String brandId) {
        this.brandId = brandId;
    }

    public String getSeasonId() {
        return seasonId;
    }

    public void setSeasonId(String seasonId) {
        this.seasonId = this.seasonId;
    }

    public String getProductCategoryId() {
        return productCategoryId;
    }

    public void setProductCategoryId(String productCategoryId) {
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

    public void setWarrantyId(String warrantyId) {
        this.warrantyId = warrantyId;
    }

    public String getShippingClassId() {
        return shippingClassId;
    }

    public void setShippingClassId(String shippingClassId) {
        this.shippingClassId = shippingClassId;
    }

    public String getOfficialStoreId() {
        return officialStoreId;
    }

    public void setOfficialStoreId(String officialStoreId) {
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

    public void setUpdatedById(String updatedById) {
        this.updatedById = updatedById;
    }

    public String getMerchantId() {
        return merchantId;
    }

    public void setMerchantId(String merchantId) {
        this.merchantId = merchantId;
    }

    public Date getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(Date updatedAt) {
        this.updatedAt = updatedAt;
    }

    public Date getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(Date createdAt) {
        this.createdAt = createdAt;
    }

    public String getProductTypeId() {
        return productTypeId;
    }

    public void setProductTypeId(String productTypeId) {
        this.productTypeId = productTypeId;
    }

    public String getIntegrationCompany(){ return this.integrationCompany; }

    public void setIntegrationCompany(String integrationCompany){ this.integrationCompany = integrationCompany; }

    public String getVegSyncStatus(){ return this.vegSyncStatus; }

    public void setVegSyncStatus(String vegSyncStatus){ this.vegSyncStatus = vegSyncStatus; }

    public String getDefaultVersionId(){ return this.defaultVersionId; }

    public void setDefaultVersionId(String defaultVersionId){ this.defaultVersionId = defaultVersionId; }

   // public SyncProductVersion[] getSyncProductVersions(){ return  this.syncProductVersions; }

    public Company getCompany() { return company; }

    public void setCompany(Company company) {
        this.company = company;
    }
}