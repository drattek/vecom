package com.vegusa.middleware.entity;

import jakarta.persistence.*;
import org.hibernate.annotations.ColumnDefault;

@Entity
@Table(name = "productattribute")
public class ProductAttribute {
    @EmbeddedId
    private ProductAttributeId id;

    @Column(name = "Name", length = 100)
    private String name;

    @ColumnDefault("'Text'")
    @Lob
    @Column(name = "DataType", nullable = false)
    private String dataType;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    private Company company;

    public ProductAttributeId getId() {
        return id;
    }

    public void setId(ProductAttributeId id) {
        this.id = id;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getDataType() {
        return dataType;
    }

    public void setDataType(String dataType) {
        this.dataType = dataType;
    }

    public Company getCompany() {
        return company;
    }

    public void setCompany(Company company) {
        this.company = company;
    }

}