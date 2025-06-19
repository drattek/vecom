package com.vegusa.middleware.entity;

import com.fasterxml.jackson.annotation.JsonInclude;
import jakarta.persistence.*;
import org.hibernate.annotations.ColumnDefault;

import java.time.Instant;
import java.util.Date;

@Entity
@Table(name = "product")
@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class InterfaceItems {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long id;

    @Column(name = "ItemId", nullable = false, length = 50)
    private String itemId;

    @Column(name = "ProductName", length = 150)
    private String productName;

    @Column(name = "ShortDescription", length = 500)
    private String shortDescription;

    @Column(name = "PartNumber", nullable = false, length = 100)
    private String partNumber;

    @Column(name = "Weight", length = 20)
    private String weight;

    @Column(name = "UnitOfMeasurement", length = 20)
    private String unitOfMeasurement;

    @Column(name = "Category", length = 500)
    private String category;

    @Column(name = "Brand", length = 100)
    private String brand;

    @Column(name = "Available")
    private Double available;

    @Column(name = "Cost")
    private Double cost;

    @Column(name = "CreatedAt", nullable = false)
    private Date createdAt;

    @Column(name = "UpdatedAt", nullable = false)
    private Date updatedAt;

    @ColumnDefault("'TRUE'")
    @Lob
    @Column(name = "SkipNull", nullable = false)
    private String skipNull;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "InterfaceRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "InterfaceId", referencedColumnName = "InterfaceId", nullable = false)
    })
    private Interface interfaceField;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    private Company company;

    @Column(name = "Warranty", length = 50)
    private String warranty;

    @Lob
    @Column(name = "CrossReferences")
    private String crossReferences;

    @Column(name = "Length", length = 20)
    private String length;

    @Column(name = "Height", length = 20)
    private String height;

    @Column(name = "Width", length = 20)
    private String width;

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

    public String getWeight() {
        return weight;
    }

    public void setWeight(String weight) {
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

    public Double getAvailable() {
        return available;
    }

    public void setAvailable(Double available) {
        this.available = available;
    }

    public Double getCost() {
        return cost;
    }

    public void setCost(Double cost) {
        this.cost = cost;
    }

    public Date getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(Date createdAt) {
        this.createdAt = createdAt;
    }

    public Date getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(Date updatedAt) {
        this.updatedAt = updatedAt;
    }

    public String getSkipNull() {
        return skipNull;
    }

    public void setSkipNull(String skipNull) {
        this.skipNull = skipNull;
    }

    public Interface getInterfaceField() {
        return interfaceField;
    }

    public void setInterfaceField(Interface interfaceField) {
        this.interfaceField = interfaceField;
    }

    public Company getCompany() {
        return company;
    }

    public void setCompany(Company company) {
        this.company = company;
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

    public String getLength() {
        return length;
    }

    public void setLength(String length) {
        this.length = length;
    }

    public String getHeight() {
        return height;
    }

    public void setHeight(String height) {
        this.height = height;
    }

    public String getWidth() {
        return width;
    }

    public void setWidth(String width) {
        this.width = width;
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