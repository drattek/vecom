package com.vegusa.middleware.entity;

import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.persistence.*;
import java.util.Date;

@Entity
@Table(name = "SyncBrand")
public class SyncBrand {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long recId;

    @Column(name = "ResponseId", length = 100)
    @JsonProperty("_id")
    private String responseId;

    @Column(name = "Name", length = 150)
    @JsonProperty("name")
    private String name;

    @Column(name = "Code", length = 100)
    @JsonProperty("code")
    private String code;

    @Column(name = "Description", length = 150)
    @JsonProperty("description")
    private String description;

    @Column(name = "Tags", length = 100)
    @JsonProperty("tags")
    private String tags;

    @Column(name = "ResponseStatus", length = 100)
    @JsonProperty("status")
    private String responseStatus;

    @Column(name = "CreatedAt")
    @JsonProperty("createdAt")
    private Date createdAt;

    @Column(name = "UpdatedAt")
    @JsonProperty("updatedAt")
    private Date updatedAt;

    @Column(name = "CreatedById", length = 100)
    @JsonProperty("CreatedById")
    private String createdById;

    @Column(name = "UpdatedById", length = 100)
    @JsonProperty("UpdatedById")
    private String updatedById;

    @Column(name = "MerchantId", length = 100)
    @JsonProperty("MerchantId")
    private String merchantId;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    private Company company;

    public Long getRecId() {
        return recId;
    }

    public void setRecId(Long recId) {
        this.recId = recId;
    }

    public String getResponseId() {
        return responseId;
    }

    public void setResponseId(String responseId) {
        this.responseId = responseId;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getCode() {
        return code;
    }

    public void setCode(String code) {
        this.code = code;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getTags() {
        return tags;
    }

    public void setTags(String tags) {
        this.tags = tags;
    }

    public String getResponseStatus() {
        return responseStatus;
    }

    public void setResponseStatus(String responseStatus) {
        this.responseStatus = this.responseStatus;
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

    public Company getCompany() {
        return company;
    }

    public void setCompany(Company company) {
        this.company = company;
    }
}