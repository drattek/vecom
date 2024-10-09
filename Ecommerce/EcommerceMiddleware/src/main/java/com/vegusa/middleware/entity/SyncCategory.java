package com.vegusa.middleware.entity;

import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.persistence.*;

import java.util.Date;

@Entity
@Table(name = "SyncCategory")
public class SyncCategory {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "id", nullable = false)
    private Long id;

    @Column(name = "id_ecom", length = 100)
    @JsonProperty("_id")
    private String idEcom;

    @Column(name = "name", length = 250)
    private String name;

    @Column(name = "branch", length = 250)
    private String branch;

    @Column(name = "description", length = 150)
    private String description;

    @Column(name = "status_ecom", length = 100)
    @JsonProperty("status")
    private String statusEcom;

    @Column(name = "created_at")
    @JsonProperty("createdAt")
    private Date createdAt;

    @Column(name = "updated_at")
    @JsonProperty("updatedAt")
    private Date updatedAt;

    @Column(name = "created_by_id", length = 100)
    @JsonProperty("CreatedById")
    private String createdById;

    @Column(name = "updated_by_id", length = 100)
    @JsonProperty("UpdatedById")
    private String updatedById;

    @Column(name = "merchant_id", length = 100)
    @JsonProperty("MerchantId")
    private String merchantId;

    @Column(name = "veg_company", nullable = false, length = 50)
    private String vegCompany;

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getIdEcom() {
        return idEcom;
    }

    public void setIdEcom(String idEcom) {
        this.idEcom = idEcom;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getBranch() {
        return branch;
    }

    public void setBranch(String branch) {
        this.branch = branch;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getStatusEcom() {
        return statusEcom;
    }

    public void setStatusEcom(String statusEcom) {
        this.statusEcom = statusEcom;
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

    public String getVegCompany() {
        return vegCompany;
    }

    public void setVegCompany(String vegCompany) {
        this.vegCompany = vegCompany;
    }

}