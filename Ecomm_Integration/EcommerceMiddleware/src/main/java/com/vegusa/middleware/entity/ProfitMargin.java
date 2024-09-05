package com.vegusa.middleware.entity;

import jakarta.persistence.*;
import org.hibernate.annotations.ColumnDefault;

import java.math.BigDecimal;

@Entity
@Table(name = "profitmargin")
public class ProfitMargin {
    @EmbeddedId
    private ProfitMarginId id;

    @Column(name = "Percentage", nullable = false, precision = 4, scale = 1)
    private BigDecimal percentage;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CategoryRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "CategoryName", referencedColumnName = "Name", nullable = false)
    })
    private ProfitMarginCategory profitmargincategory;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    private Company company;

    @ColumnDefault("'MXN'")
    @Lob
    @Column(name = "CurrencyCode", nullable = false)
    private String currencyCode;

    public ProfitMarginId getId() {
        return id;
    }

    public void setId(ProfitMarginId id) {
        this.id = id;
    }

    public BigDecimal getPercentage() {
        return percentage;
    }

    public void setPercentage(BigDecimal percentage) {
        this.percentage = percentage;
    }

    public ProfitMarginCategory getProfitmargincategory() {
        return profitmargincategory;
    }

    public void setProfitmargincategory(ProfitMarginCategory profitmargincategory) {
        this.profitmargincategory = profitmargincategory;
    }

    public Company getCompany() {
        return company;
    }

    public void setCompany(Company company) {
        this.company = company;
    }

    public String getCurrencyCode() {
        return currencyCode;
    }

    public void setCurrencyCode(String currencyCode) {
        this.currencyCode = currencyCode;
    }

}