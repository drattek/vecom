package com.vegusa.middleware.integrations.jumpseller.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "syncItemJumpseller")
public class SyncJumpsellerProduct {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId")
    private Long recId;

    @Column(name = "ResponseId", unique = true)
    private Long responseId;

    @Column(name = "InternalCode")
    private String internalCode;

    @Column(name = "Name")
    private String name;

    @Column(name = "PageTitle")
    private String pageTitle;

    @Column(name = "Description", columnDefinition = "TEXT")
    private String description;

    @Column(name = "MetaDescription", columnDefinition = "TEXT")
    private String metaDescription;

    @Column(name = "Type")
    private String type;

    @Column(name = "DaysToExpire")
    private int daysToExpire;

    @Column(name = "Price")
    private double price;

    @Column(name = "Discount")
    private double discount;

    @Column(name = "Weight")
    private double weight;

    @Column(name = "Stock")
    private int stock;

    @Column(name = "StockUnlimited")
    private boolean stockUnlimited;

    @Column(name = "StockThreshold")
    private int stockThreshold;

    @Column(name = "StockNotification")
    private boolean stockNotification;

    @Column(name = "CostPerItem")
    private Double costPerItem;

    @Column(name = "CompareAtPrice")
    private Double compareAtPrice;

    @Column(name = "MinimumQuantity")
    private int minimumQuantity;

    @Column(name = "MaximumQuantity")
    private int maximumQuantity;

    @Column(name = "Sku")
    private String sku;

    @Column(name = "Brand")
    private String brand;

    @Column(name = "Barcode")
    private String barcode;

    @Column(name = "GoogleProductCategory")
    private String googleProductCategory;

    @Column(name = "Featured")
    private boolean featured;

    @Column(name = "ShippingRequired")
    private boolean shippingRequired;

    @Column(name = "ReviewsEnabled")
    private boolean reviewsEnabled;

    @Column(name = "Status")
    private String status;

    @Column(name = "CreatedAt")
    private String createdAt;

    @Column(name = "UpdatedAt")
    private String updatedAt;

    @Column(name = "PackageFormat")
    private String packageFormat;

    @Column(name = "Length")
    private double length;

    @Column(name = "Width")
    private double width;

    @Column(name = "Height")
    private double height;

    @Column(name = "Diameter")
    private double diameter;

    @Column(name = "Permalink")
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

    public double getPrice() {
        return price;
    }

    public void setPrice(double price) {
        this.price = price;
    }

    public double getDiscount() {
        return discount;
    }

    public void setDiscount(double discount) {
        this.discount = discount;
    }

    public double getWeight() {
        return weight;
    }

    public void setWeight(double weight) {
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

    public double getLength() {
        return length;
    }

    public void setLength(double length) {
        this.length = length;
    }

    public double getWidth() {
        return width;
    }

    public void setWidth(double width) {
        this.width = width;
    }

    public double getHeight() {
        return height;
    }

    public void setHeight(double height) {
        this.height = height;
    }

    public double getDiameter() {
        return diameter;
    }

    public void setDiameter(double diameter) {
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
