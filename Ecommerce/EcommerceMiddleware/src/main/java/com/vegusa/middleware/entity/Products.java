package com.vegusa.middleware.entity;

import jakarta.persistence.*;

import java.math.BigDecimal;
import java.time.Instant;

@Entity
@Table(name = "product")
public class Products {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RectId")
    private Long id;

    @Column(name = "ItemId")
    private String itemId;

    @Column(name = "ProductName")
    private String productName;

    @Column(name = "ShortDescription")
    private String shortDescription;

    @Column(name = "PartNumber")
    private String partNumber;

    @Column(name = "Weight")
    private BigDecimal weight;

    @Column(name = "UnitOfMeasurement")
    private String unitOfMeasurement;

    @Column(name = "Category")
    private String category;

    @Column(name = "Brand")
    private String brand;

    @Column(name = "Available")
    private Long available;

    @Column(name = "Cost")
    private BigDecimal cost;

    @Column(name = "CreatedAt")
    private Instant createdAt;

    @Column(name = "UpdatedAt")
    private Instant updatedAt;

    @Column(name = "SkipNull")
    private String skipNull;

    @Column(name = "InterfaceId")
    private String interfaceId;

    @Column(name = "DataAreaId")
    private String dataAreaId;

    @Column(name = "InterfaceRefRecId")
    private Long interfaceRefRecId;

    @Column(name = "CompanyRefRecId")
    private Long companyRefRecId;

    @Column(name = "Warranty")
    private String warranty;

    @Column(name = "CrossReferences")
    private String crossReferences;

    @Column(name = "Length")
    private BigDecimal length;

    @Column(name = "Height")
    private BigDecimal height;

    @Column(name = "Width")
    private BigDecimal width;

    @Column(name = "DangerGoodsRegulation")
    private String dangerGoodsRegulation;

    @Column(name = "CountryOfOrigin")
    private String countryOfOrigin;

    @Column(name = "ProductCondition")
    private String productCondition;

    @Column(name = "SeoTitle")
    private String seoTitle;

    @Column(name = "MetaDescription")
    private String metaDescription;

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getItemId() {
        return itemId;
    }

    public void setItemId(String itemId) {
        this.itemId = itemId;
    }

    public String getProductName() {
        return productName;
    }

    public void setProductName(String productName) {
        this.productName = productName;
    }

    public String getShortDescription() {
        return shortDescription;
    }

    public void setShortDescription(String shortDescription) {
        this.shortDescription = shortDescription;
    }

    public String getPartNumber() {
        return partNumber;
    }

    public void setPartNumber(String partNumber) {
        this.partNumber = partNumber;
    }

    public BigDecimal getWeight() {
        return weight;
    }

    public void setWeight(BigDecimal weight) {
        this.weight = weight;
    }

    public String getUnitOfMeasurement() {
        return unitOfMeasurement;
    }

    public void setUnitOfMeasurement(String unitOfMeasurement) {
        this.unitOfMeasurement = unitOfMeasurement;
    }

    public String getCategory() {
        return category;
    }

    public void setCategory(String category) {
        this.category = category;
    }

    public String getBrand() {
        return brand;
    }

    public void setBrand(String brand) {
        this.brand = brand;
    }

    public Long getAvailable() {
        return available;
    }

    public void setAvailable(Long available) {
        this.available = available;
    }

    public BigDecimal getCost() {
        return cost;
    }

    public void setCost(BigDecimal cost) {
        this.cost = cost;
    }

    public Instant getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(Instant createdAt) {
        this.createdAt = createdAt;
    }

    public Instant getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(Instant updatedAt) {
        this.updatedAt = updatedAt;
    }

    public String getSkipNull() {
        return skipNull;
    }

    public void setSkipNull(String skipNull) {
        this.skipNull = skipNull;
    }

    public String getInterfaceId() {
        return interfaceId;
    }

    public void setInterfaceId(String interfaceId) {
        this.interfaceId = interfaceId;
    }

    public String getDataAreaId() {
        return dataAreaId;
    }

    public void setDataAreaId(String dataAreaId) {
        this.dataAreaId = dataAreaId;
    }

    public Long getInterfaceRefRecId() {
        return interfaceRefRecId;
    }

    public void setInterfaceRefRecId(Long interfaceRefRecId) {
        this.interfaceRefRecId = interfaceRefRecId;
    }

    public Long getCompanyRefRecId() {
        return companyRefRecId;
    }

    public void setCompanyRefRecId(Long companyRefRecId) {
        this.companyRefRecId = companyRefRecId;
    }

    public String getWarranty() {
        return warranty;
    }

    public void setWarranty(String warranty) {
        this.warranty = warranty;
    }

    public String getCrossReferences() {
        return crossReferences;
    }

    public void setCrossReferences(String crossReferences) {
        this.crossReferences = crossReferences;
    }

    public BigDecimal getLength() {
        return length;
    }

    public void setLength(BigDecimal length) {
        this.length = length;
    }

    public BigDecimal getHeight() {
        return height;
    }

    public void setHeight(BigDecimal height) {
        this.height = height;
    }

    public BigDecimal getWidth() {
        return width;
    }

    public void setWidth(BigDecimal width) {
        this.width = width;
    }

    public String getDangerGoodsRegulation() {
        return dangerGoodsRegulation;
    }

    public void setDangerGoodsRegulation(String dangerGoodsRegulation) {
        this.dangerGoodsRegulation = dangerGoodsRegulation;
    }

    public String getCountryOfOrigin() {
        return countryOfOrigin;
    }

    public void setCountryOfOrigin(String countryOfOrigin) {
        this.countryOfOrigin = countryOfOrigin;
    }

    public String getProductCondition() {
        return productCondition;
    }

    public void setProductCondition(String productCondition) {
        this.productCondition = productCondition;
    }

    public String getSeoTitle() {
        return seoTitle;
    }

    public void setSeoTitle(String seoTitle) {
        this.seoTitle = seoTitle;
    }

    public String getMetaDescription() {
        return metaDescription;
    }

    public void setMetaDescription(String metaDescription) {
        this.metaDescription = metaDescription;
    }
}
