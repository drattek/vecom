package com.vegusa.middleware.entity;

import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.persistence.*;

import java.util.Date;

@Entity
@Table(name = "synctag")
public class SyncTag {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long id;

    @Column(name = "ResponseId", length = 100)
    @JsonProperty("_id")
    private String responseId;

    @Column(name = "ResponseStatus", length = 100)
    @JsonProperty("status")
    private String responseStatus;

    @Column(name = "Name", length = 250)
    @JsonProperty("name")
    private String name;

    @Column(name = "Slug", length = 250)
    @JsonProperty("slug")
    private String slug;

    @Column(name = "Description", length = 250)
    @JsonProperty("description")
    private String description;

    @Column(name = "TagTypeId", length = 100)
    @JsonProperty("TagTypeId")
    private String tagTypeId;

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

    public String getResponseStatus() {
        return responseStatus;
    }

    public void setResponseStatus(String responseStatus) {
        this.responseStatus = responseStatus;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getSlug() {
        return slug;
    }

    public void setSlug(String slug) {
        this.slug = slug;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getTagTypeId() {
        return tagTypeId;
    }

    public void setTagTypeId(String tagTypeId) {
        this.tagTypeId = tagTypeId;
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