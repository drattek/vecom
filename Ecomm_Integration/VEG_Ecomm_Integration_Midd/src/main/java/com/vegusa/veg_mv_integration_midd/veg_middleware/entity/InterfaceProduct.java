package com.vegusa.veg_mv_integration_midd.veg_middleware.entity;

import jakarta.persistence.*;
import org.hibernate.annotations.ColumnDefault;

import java.time.Instant;
import java.util.Date;

@Entity
@Table(name = "product")
public class InterfaceProduct {
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

    @Column(name = "Category", length = 50)
    private String category;

    @Column(name = "Brand", length = 50)
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
    @Column(name = "SkipNull")
    private String skipNull;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "InterfaceRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "InterfaceId", referencedColumnName = "InterfaceId", nullable = false)
    })
    private InterfaceDS interfaceField;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    private Company company;

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

    public void setUpdatedAt(Date modifiedAt) {
        this.updatedAt = modifiedAt;
    }

    public String getSkipNull() {
        return skipNull;
    }

    public void setSkipNull(String skipNull) {
        this.skipNull = skipNull;
    }

    public InterfaceDS getInterfaceField() {
        return interfaceField;
    }

    public void setInterfaceField(InterfaceDS interfaceField) {
        this.interfaceField = interfaceField;
    }

    public Company getCompany() {
        return company;
    }

    public void setCompany(Company company) {
        this.company = company;
    }

}