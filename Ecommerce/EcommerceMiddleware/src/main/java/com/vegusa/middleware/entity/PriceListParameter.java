package com.vegusa.middleware.entity;

import jakarta.persistence.*;
import org.hibernate.annotations.ColumnDefault;

import java.math.BigDecimal;

@Entity
@Table(name = "pricelistparameter")
public class PriceListParameter {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", columnDefinition = "int UNSIGNED not null")
    private Long id;

    @Column(name = "Name", nullable = false, length = 50)
    private String name;

    @ColumnDefault("'string'")
    @Lob
    @Column(name = "DataType")
    private String dataType;

    @Column(name = "StrValue", length = 250)
    private String strValue;

    @Column(name = "IntValue")
    private Integer intValue;

    @Column(name = "DecValue", precision = 12, scale = 2)
    private BigDecimal decValue;

    @Column(name = "Description", length = 250)
    private String description;

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

    public String getStrValue() {
        return strValue;
    }

    public void setStrValue(String strValue) {
        this.strValue = strValue;
    }

    public Integer getIntValue() {
        return intValue;
    }

    public void setIntValue(Integer intValue) {
        this.intValue = intValue;
    }

    public BigDecimal getDecValue() {
        return decValue;
    }

    public void setDecValue(BigDecimal decValue) {
        this.decValue = decValue;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public Company getCompany() {
        return company;
    }

    public void setCompany(Company company) {
        this.company = company;
    }

}