package com.vegusa.middleware.entity;

import jakarta.persistence.*;

import java.time.Instant;

@Entity
@Table(name = "productattributevalue")
public class ProductAttributeValues {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId")
    private Long id;

    @Column(name = "ItemId")
    private String itemId;

    @Column(name = "InterfaceId")
    private String interfaceId;

    @Column(name = "ProductAttributeId")
    private String productAttributeId;

    @Column(name = "Value")
    private String value;

    @Column(name = "SkipNull")
    private String skipNull;

    @Column(name = "CreatedAt")
    private Instant createdAt;

    @Column(name = "UpdatedAt")
    private Instant updatedAt;

    @Column(name = "DataAreaId")
    private String dataAreaId;

    @Column(name = "InterfaceRefRecId")
    private Long interfaceRefRecId;

    @Column(name = "ProductAttributeRefRecId")
    private Long productAttributeRefRecId;

    @Column(name = "CompanyRefRecId")
    private Long companyRefRecId;

    @Column(name = "ProductRefRecId")
    private Long productRefRecId;

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

    public String getInterfaceId() {
        return interfaceId;
    }

    public void setInterfaceId(String interfaceId) {
        this.interfaceId = interfaceId;
    }

    public String getProductAttributeId() {
        return productAttributeId;
    }

    public void setProductAttributeId(String productAttributeId) {
        this.productAttributeId = productAttributeId;
    }

    public String getValue() {
        return value;
    }

    public void setValue(String value) {
        this.value = value;
    }

    public String getSkipNull() {
        return skipNull;
    }

    public void setSkipNull(String skipNull) {
        this.skipNull = skipNull;
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

    public Long getProductAttributeRefRecId() {
        return productAttributeRefRecId;
    }

    public void setProductAttributeRefRecId(Long productAttributeRefRecId) {
        this.productAttributeRefRecId = productAttributeRefRecId;
    }

    public Long getCompanyRefRecId() {
        return companyRefRecId;
    }

    public void setCompanyRefRecId(Long companyRefRecId) {
        this.companyRefRecId = companyRefRecId;
    }

    public Long getProductRefRecId() {
        return productRefRecId;
    }

    public void setProductRefRecId(Long productRefRecId) {
        this.productRefRecId = productRefRecId;
    }
}
