package com.vegusa.middleware.entity;

import jakarta.persistence.*;

import java.time.Instant;

@Entity
@Table(name = "integration_images")
public class IntegrationImage {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "id", nullable = false)
    private Long id;

    @Column(name = "product_id")
    private Long productId;

    @Column(name = "internal_code")
    private String internalCode;

    @Column(name = "product_external_id")
    private String productExternalId;

    @Column(name = "external_id")
    private String externalId;

    @Column(name = "url")
    private String url;

    @Column(name = "external_url")
    private String externalUrl;

    @Column(name = "position")
    private Long position;

    @Column(name = "file_name")
    private String fileName;

    @Column(name = "data_area_id")
    private String dataAreaId;

    @Column(name = "integration_name")
    private String integrationName;

    @Column(name = "integration_parameter_id")
    private Long integrationParameterId;

    @Column(name = "company_id")
    private Long companyId;

    @Column(name = "created_at")
    private Instant createdAt;

    @Column(name = "updated_at")
    private Instant updatedAt;

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public Long getProductId() {
        return productId;
    }

    public void setProductId(Long productId) {
        this.productId = productId;
    }

    public String getInternalCode() {
        return internalCode;
    }

    public void setInternalCode(String internalCode) {
        this.internalCode = internalCode;
    }

    public String getProductExternalId() {
        return productExternalId;
    }

    public void setProductExternalId(String productExternalId) {
        this.productExternalId = productExternalId;
    }

    public String getExternalId() {
        return externalId;
    }

    public void setExternalId(String externalId) {
        this.externalId = externalId;
    }

    public String getUrl() {
        return url;
    }

    public void setUrl(String url) {
        this.url = url;
    }

    public String getExternalUrl() {
        return externalUrl;
    }

    public void setExternalUrl(String externalUrl) {
        this.externalUrl = externalUrl;
    }

    public Long getPosition() {
        return position;
    }

    public void setPosition(Long position) {
        this.position = position;
    }

    public String getFileName() {
        return fileName;
    }

    public void setFileName(String fileName) {
        this.fileName = fileName;
    }

    public String getDataAreaId() {
        return dataAreaId;
    }

    public void setDataAreaId(String dataAreaId) {
        this.dataAreaId = dataAreaId;
    }

    public String getIntegrationName() {
        return integrationName;
    }

    public void setIntegrationName(String integrationName) {
        this.integrationName = integrationName;
    }

    public Long getIntegrationParameterId() {
        return integrationParameterId;
    }

    public void setIntegrationParameterId(Long integrationParameterId) {
        this.integrationParameterId = integrationParameterId;
    }

    public Long getCompanyId() {
        return companyId;
    }

    public void setCompanyId(Long companyId) {
        this.companyId = companyId;
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
}
