package com.vegusa.middleware.integrations.jumpseller.entity;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.persistence.*;

import java.math.BigDecimal;
import java.time.Instant;

@Entity
@Table(name = "syncItemJumpseller")
@JsonIgnoreProperties(ignoreUnknown = true)
public class SyncJumpsellerProduct {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId")
    private Long recId;

    @Column(name = "ResponseId", unique = true)
    @JsonProperty("id")
    private Long responseId;

    @Column(name = "InternalCode")
    private String internalCode;

    @Column(name = "Name")
    @JsonProperty("name")
    private String name;

    @Column(name = "PageTitle")
    @JsonProperty("page_title")
    private String pageTitle;

    @Column(name = "Description", columnDefinition = "TEXT")
    @JsonProperty("description")
    private String description;

    @Column(name = "MetaDescription", columnDefinition = "TEXT")
    @JsonProperty("meta_description")
    private String metaDescription;

    @Column(name = "Type")
    @JsonProperty("type")
    private String type;

    @Column(name = "DaysToExpire")
    @JsonProperty("days_to_expire")
    private int daysToExpire;

    @Column(name = "Price")
    @JsonProperty("price")
    private BigDecimal price;

    @Column(name = "Discount")
    @JsonProperty("discount")
    private double discount;

    @Column(name = "Weight")
    @JsonProperty("weight")
    private BigDecimal weight;

    @Column(name = "Stock")
    @JsonProperty("stock")
    private int stock;

    @Column(name = "StockUnlimited")
    @JsonProperty("stock_unlimited")
    private boolean stockUnlimited;

    @Column(name = "StockThreshold")
    @JsonProperty("stock_threshold")
    private int stockThreshold;

    @Column(name = "StockNotification")
    @JsonProperty("stock_notification")
    private boolean stockNotification;

    @Column(name = "CostPerItem")
    @JsonProperty("cost_per_item")
    private Double costPerItem;

    @Column(name = "CompareAtPrice")
    @JsonProperty("compare_at_price")
    private Double compareAtPrice;

    @Column(name = "MinimumQuantity")
    @JsonProperty("minimum_quantity")
    private int minimumQuantity;

    @Column(name = "MaximumQuantity")
    @JsonProperty("maximum_quantity")
    private int maximumQuantity;

    @Column(name = "Sku")
    @JsonProperty("sku")
    private String sku;

    @Column(name = "Brand")
    @JsonProperty("brand")
    private String brand;

    @Column(name = "Barcode")
    @JsonProperty("barcode")
    private String barcode;

    @Column(name = "GoogleProductCategory")
    @JsonProperty("google_product_category")
    private String googleProductCategory;

    @Column(name = "Featured")
    @JsonProperty("featured")
    private boolean featured;

    @Column(name = "ShippingRequired")
    @JsonProperty("shipping_required")
    private boolean shippingRequired;

    @Column(name = "ReviewsEnabled")
    @JsonProperty("reviews_enabled")
    private boolean reviewsEnabled;

    @Column(name = "Status")
    @JsonProperty("status")
    private String status;

    @Column(name = "CreatedAt")
    @JsonProperty("created_at")
    private String createdAt;

    @Column(name = "UpdatedAt")
    @JsonProperty("updated_at")
    private String updatedAt;

    @Column(name = "PackageFormat")
    @JsonProperty("package_format")
    private String packageFormat;

    @Column(name = "Length")
    @JsonProperty("length")
    private BigDecimal length;

    @Column(name = "Width")
    @JsonProperty("width")
    private BigDecimal width;

    @Column(name = "Height")
    @JsonProperty("height")
    private BigDecimal height;

    @Column(name = "Diameter")
    @JsonProperty("diameter")
    private BigDecimal diameter;

    @Column(name = "Permalink")
    @JsonProperty("permalink")
    private String permalink;

    @Column(name = "DataAreaId")
    private String dataAreaId;

    @Column(name = "CompanyRefRecId")
    private Long companyRefRecId;

    public Long getRecId() {
        return recId;
    }

    public void setRecId(Long recId) {
        this.recId = recId;
    }

    public Long getResponseId() {
        return responseId;
    }

    public void setResponseId(Long responseId) {
        this.responseId = responseId;
    }

    public String getInternalCode() {
        return internalCode;
    }

    public void setInternalCode(String internalCode) {
        this.internalCode = internalCode;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getPageTitle() {
        return pageTitle;
    }

    public void setPageTitle(String pageTitle) {
        this.pageTitle = pageTitle;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getMetaDescription() {
        return metaDescription;
    }

    public void setMetaDescription(String metaDescription) {
        this.metaDescription = metaDescription;
    }

    public String getType() {
        return type;
    }

    public void setType(String type) {
        this.type = type;
    }

    public int getDaysToExpire() {
        return daysToExpire;
    }

    public void setDaysToExpire(int daysToExpire) {
        this.daysToExpire = daysToExpire;
    }

    public BigDecimal getPrice() {
        return price;
    }

    public void setPrice(BigDecimal price) {
        this.price = price;
    }

    public double getDiscount() {
        return discount;
    }

    public void setDiscount(double discount) {
        this.discount = discount;
    }

    public BigDecimal getWeight() {
        return weight;
    }

    public void setWeight(BigDecimal weight) {
        this.weight = weight;
    }

    public int getStock() {
        return stock;
    }

    public void setStock(int stock) {
        this.stock = stock;
    }

    public boolean isStockUnlimited() {
        return stockUnlimited;
    }

    public void setStockUnlimited(boolean stockUnlimited) {
        this.stockUnlimited = stockUnlimited;
    }

    public int getStockThreshold() {
        return stockThreshold;
    }

    public void setStockThreshold(int stockThreshold) {
        this.stockThreshold = stockThreshold;
    }

    public boolean isStockNotification() {
        return stockNotification;
    }

    public void setStockNotification(boolean stockNotification) {
        this.stockNotification = stockNotification;
    }

    public Double getCostPerItem() {
        return costPerItem;
    }

    public void setCostPerItem(Double costPerItem) {
        this.costPerItem = costPerItem;
    }

    public Double getCompareAtPrice() {
        return compareAtPrice;
    }

    public void setCompareAtPrice(Double compareAtPrice) {
        this.compareAtPrice = compareAtPrice;
    }

    public int getMinimumQuantity() {
        return minimumQuantity;
    }

    public void setMinimumQuantity(int minimumQuantity) {
        this.minimumQuantity = minimumQuantity;
    }

    public int getMaximumQuantity() {
        return maximumQuantity;
    }

    public void setMaximumQuantity(int maximumQuantity) {
        this.maximumQuantity = maximumQuantity;
    }

    public String getSku() {
        return sku;
    }

    public void setSku(String sku) {
        this.sku = sku;
    }

    public String getBrand() {
        return brand;
    }

    public void setBrand(String brand) {
        this.brand = brand;
    }

    public String getBarcode() {
        return barcode;
    }

    public void setBarcode(String barcode) {
        this.barcode = barcode;
    }

    public String getGoogleProductCategory() {
        return googleProductCategory;
    }

    public void setGoogleProductCategory(String googleProductCategory) {
        this.googleProductCategory = googleProductCategory;
    }

    public boolean isFeatured() {
        return featured;
    }

    public void setFeatured(boolean featured) {
        this.featured = featured;
    }

    public boolean isShippingRequired() {
        return shippingRequired;
    }

    public void setShippingRequired(boolean shippingRequired) {
        this.shippingRequired = shippingRequired;
    }

    public boolean isReviewsEnabled() {
        return reviewsEnabled;
    }

    public void setReviewsEnabled(boolean reviewsEnabled) {
        this.reviewsEnabled = reviewsEnabled;
    }

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
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

    public String getPackageFormat() {
        return packageFormat;
    }

    public void setPackageFormat(String packageFormat) {
        this.packageFormat = packageFormat;
    }

    public BigDecimal getLength() {
        return length;
    }

    public void setLength(BigDecimal length) {
        this.length = length;
    }

    public BigDecimal getWidth() {
        return width;
    }

    public void setWidth(BigDecimal width) {
        this.width = width;
    }

    public BigDecimal getHeight() {
        return height;
    }

    public void setHeight(BigDecimal height) {
        this.height = height;
    }

    public BigDecimal getDiameter() {
        return diameter;
    }

    public void setDiameter(BigDecimal diameter) {
        this.diameter = diameter;
    }

    public String getPermalink() {
        return permalink;
    }

    public void setPermalink(String permalink) {
        this.permalink = permalink;
    }

    public String getDataAreaId() {
        return dataAreaId;
    }

    public void setDataAreaId(String dataAreaId) {
        this.dataAreaId = dataAreaId;
    }

    public Long getCompanyRefRecId() {
        return companyRefRecId;
    }

    public void setCompanyRefRecId(Long companyRefRecId) {
        this.companyRefRecId = companyRefRecId;
    }
}
