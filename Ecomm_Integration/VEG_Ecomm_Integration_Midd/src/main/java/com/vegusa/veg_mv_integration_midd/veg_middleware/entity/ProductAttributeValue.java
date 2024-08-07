package com.vegusa.veg_mv_integration_midd.veg_middleware.entity;

import jakarta.persistence.*;
import org.hibernate.annotations.ColumnDefault;

import java.time.Instant;
import java.util.Date;

@Entity
@Table(name = "productattributevalue")
public class ProductAttributeValue {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long id;

    @Column(name = "ItemId", nullable = false, length = 50)
    private String itemId;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "InterfaceRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "InterfaceId", referencedColumnName = "InterfaceId", nullable = false)
    })
    private InterfaceDS interfaceField;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "ProductAttributeRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "ProductAttributeId", referencedColumnName = "ProductAttributeId", nullable = false)
    })
    private ProductAttribute productattribute;

    @Column(name = "Value", length = 250)
    private String value;

    @ColumnDefault("'TRUE'")
    @Lob
    @Column(name = "SkipNull", nullable = false)
    private String skipNull;

    @Column(name = "CreatedAt", nullable = false)
    private Date createdAt;

    @Column(name = "UpdatedAt", nullable = false)
    private Date updatedAt;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    private Company company;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "ProductRefRecId", nullable = false)
    private InterfaceProduct productRefRec;

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

    public InterfaceDS getInterfaceField() {
        return interfaceField;
    }

    public void setInterfaceField(InterfaceDS interfaceField) {
        this.interfaceField = interfaceField;
    }

    public ProductAttribute getProductattribute() {
        return productattribute;
    }

    public void setProductattribute(ProductAttribute productattribute) {
        this.productattribute = productattribute;
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

    public Company getCompany() {
        return company;
    }

    public void setCompany(Company company) {
        this.company = company;
    }

    public InterfaceProduct getProductRefRec() {
        return productRefRec;
    }

    public void setProductRefRec(InterfaceProduct productRefRec) {
        this.productRefRec = productRefRec;
    }

}