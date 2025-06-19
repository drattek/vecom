package com.vegusa.middleware.integrations.multivende.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.math.BigDecimal;
import java.util.HashMap;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class Product {
    @JsonProperty("_id")
    private String _id;
    @JsonProperty("name")
    private String name;
    @JsonProperty("code")
    private String code;
    @JsonProperty("alias")
    private String alias;
    @JsonProperty("description")
    private String description;
    @JsonProperty("htmlDescription")
    private String htmlDescription;
    @JsonProperty("shortDescription")
    private String shortDescription;
    @JsonProperty("model")
    private String model;
    @JsonProperty("internalCode")
    private String internalCode;
    @JsonProperty("createdAt")
    private String createdAt;
    @JsonProperty("updatedAt")
    private String updatedAt;
    @JsonProperty("status")
    private String status;
    @JsonProperty("BrandId")
    private String brandId;
    @JsonProperty("SeasonId")
    private String seasonId;
    @JsonProperty("CodeTypeId")
    private String codeTypeId;
    @JsonProperty("ProductTypeId")
    private String productTypeId;
    @JsonProperty("ProductCategoryId")
    private String productCategoryId;
    @JsonProperty("InternalCodeTypeId")
    private String internalCodeTypeId;
    @JsonProperty("WarrantyId")
    private String warrantyId;
    @JsonProperty("InventoryTypeId")
    private String inventoryTypeId;
    @JsonProperty("otherProductCategories")
    private String[] otherProductCategories;
    private Brand brand;
    private Season season;
    private Category productCategory;
    private Tag[] productTags;
    @JsonProperty("ProductVersions")
    private Version[] productVersions;
    @JsonProperty("CustomAttributeValues")
    private HashMap<String, String> customAttributeValues;

    public Product() {}

    public Product(String _id, String name, String code, String alias, String description, String htmlDescription, String shortDescription, String model, String internalCode, String createdAt, String updatedAt, String status, String brandId, String seasonId, String codeTypeId, String productTypeId, String productCategoryId, String internalCodeTypeId, String warrantyId, String inventoryTypeId, String[] otherProductCategories, Brand brand, Season season, Category productCategory, Tag[] productTags, Version[] productVersions, HashMap<String, String> customAttributeValues) {
        this._id = _id;
        this.name = name;
        this.code = code;
        this.alias = alias;
        this.description = description;
        this.htmlDescription = htmlDescription;
        this.shortDescription = shortDescription;
        this.model = model;
        this.internalCode = internalCode;
        this.createdAt = createdAt;
        this.updatedAt = updatedAt;
        this.status = status;
        this.brandId = brandId;
        this.seasonId = seasonId;
        this.codeTypeId = codeTypeId;
        this.productTypeId = productTypeId;
        this.productCategoryId = productCategoryId;
        this.internalCodeTypeId = internalCodeTypeId;
        this.warrantyId = warrantyId;
        this.inventoryTypeId = inventoryTypeId;
        this.otherProductCategories = otherProductCategories;
        this.brand = brand;
        this.season = season;
        this.productCategory = productCategory;
        this.productTags = productTags;
        this.productVersions = productVersions;
        this.customAttributeValues = customAttributeValues;
    }

    public String get_id() {
        return _id;
    }

    public void set_id(String _id) {
        this._id = _id;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getCode() {
        return code;
    }

    public void setCode(String code) {
        this.code = code;
    }

    public String getAlias() {
        return alias;
    }

    public void setAlias(String alias) {
        this.alias = alias;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getHtmlDescription() {
        return htmlDescription;
    }

    public void setHtmlDescription(String htmlDescription) {
        this.htmlDescription = htmlDescription;
    }

    public String getShortDescription() {
        return shortDescription;
    }

    public void setShortDescription(String shortDescription) {
        this.shortDescription = shortDescription;
    }

    public String getModel() {
        return model;
    }

    public void setModel(String model) {
        this.model = model;
    }

    public String getInternalCode() {
        return internalCode;
    }

    public void setInternalCode(String internalCode) {
        this.internalCode = internalCode;
    }

    public String getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(String createdAt) {
        this.createdAt = createdAt;
    }

    public String getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(String updatedAt) {
        this.updatedAt = updatedAt;
    }

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
    }

    public String getWarrantyId() {
        return warrantyId;
    }

    public void setWarrantyId(String warrantyId) {
        this.warrantyId = warrantyId;
    }

    public String getInventoryTypeId() {
        return inventoryTypeId;
    }

    public void setInventoryTypeId(String inventoryTypeId) {
        this.inventoryTypeId = inventoryTypeId;
    }

    public String[] getOtherProductCategories() {
        return otherProductCategories;
    }

    public void setOtherProductCategories(String[] otherProductCategories) {
        this.otherProductCategories = otherProductCategories;
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
        this.seasonId = seasonId;
    }

    public String getCodeTypeId() {
        return codeTypeId;
    }

    public void setCodeTypeId(String codeTypeId) {
        this.codeTypeId = codeTypeId;
    }

    public String getProductTypeId() {
        return productTypeId;
    }

    public void setProductTypeId(String productTypeId) {
        this.productTypeId = productTypeId;
    }

    public String getProductCategoryId() {
        return productCategoryId;
    }

    public void setProductCategoryId(String productCategoryId) {
        this.productCategoryId = productCategoryId;
    }

    public String getInternalCodeTypeId() {
        return internalCodeTypeId;
    }

    public void setInternalCodeTypeId(String internalCodeTypeId) {
        this.internalCodeTypeId = internalCodeTypeId;
    }

    public Brand getBrand() {
        return brand;
    }

    public void setBrand(Brand brand) {
        this.brand = brand;
    }

    public Season getSeason() {
        return season;
    }

    public void setSeason(Season season) {
        this.season = season;
    }

    public Category getProductCategory() {
        return productCategory;
    }

    public void setProductCategory(Category productCategory) {
        this.productCategory = productCategory;
    }

    public Tag[] getProductTags() {
        return productTags;
    }

    public void setProductTags(Tag[] productTags) {
        this.productTags = productTags;
    }

    public Version[] getProductVersions() {
        return productVersions;
    }

    public void setProductVersions(Version[] productVersions) {
        this.productVersions = productVersions;
    }

    public HashMap<String, String> getCustomAttributeValues() {
        return customAttributeValues;
    }

    public void setCustomAttributeValues(HashMap<String, String> customAttributeValues) {
        this.customAttributeValues = customAttributeValues;
    }
}
