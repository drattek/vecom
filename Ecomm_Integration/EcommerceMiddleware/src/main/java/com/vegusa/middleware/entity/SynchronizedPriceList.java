package com.vegusa.middleware.entity;

import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.persistence.*;

import java.util.Date;

@Entity
@Table(name = "synchronizedpricelist")
public class SynchronizedPriceList {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long id;

    @JsonProperty("_id")
    @Column(name = "ResponseId", length = 100)
    private String responseId;

    @JsonProperty("status")
    @Column(name = "Status", length = 20)
    private String status;

    @JsonProperty("name")
    @Column(name = "Name", length = 50)
    private String name;

    @JsonProperty("description")
    @Column(name = "Description", length = 100)
    private String description;

    @JsonProperty("CurrencyId")
    @Column(name = "CurrencyId", length = 100)
    private String currencyId;

    @JsonProperty("isDefault")
    @Column(name = "IsDefault", length = 10)
    private String isDefault;

    @JsonProperty("CreatedById")
    @Column(name = "CreatedById", length = 100)
    private String createdById;

    @JsonProperty("UpdatedById")
    @Column(name = "UpdatedById", length = 100)
    private String updatedById;

    @JsonProperty("MerchantId")
    @Column(name = "MerchantId", length = 100)
    private String merchantId;

    @JsonProperty("position")
    @Column(name = "Position", length = 10)
    private String position;

    @JsonProperty("createdAt")
    @Column(name = "CreatedAt")
    private Date createdAt;

    @JsonProperty("updatedAt")
    @Column(name = "UpdatedAt")
    private Date updatedAt;

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

    public String getResponseId() {
        return responseId;
    }

    public void setResponseId(String responseId) {
        this.responseId = responseId;
    }

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getCurrencyId() {
        return currencyId;
    }

    public void setCurrencyId(String currencyId) {
        this.currencyId = currencyId;
    }

    public String getIsDefault() {
        return isDefault;
    }

    public void setIsDefault(String isDefault) {
        this.isDefault = isDefault;
    }

    public String getCreatedById() {
        return createdById;
    }

    public void setCreatedById(String createdById) {
        this.createdById = createdById;
    }

    public String getUpdatedById() {
        return updatedById;
    }

    public void setUpdatedById(String updatedById) {
        this.updatedById = updatedById;
    }

    public String getMerchantId() {
        return merchantId;
    }

    public void setMerchantId(String merchantId) {
        this.merchantId = merchantId;
    }

    public String getPosition() {
        return position;
    }

    public void setPosition(String position) {
        this.position = position;
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

}