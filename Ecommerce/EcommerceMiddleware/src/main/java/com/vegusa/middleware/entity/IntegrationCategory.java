package com.vegusa.middleware.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "integration_category")
public class IntegrationCategory {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long id;

    @Column(name = "CategoryId")
    private Long categoryId;

    @Column(name = "ExternalId")
    private String externalId;

    @Column(name = "Name")
    private String Name;

    @Column(name = "ExternalName")
    private String externalName;

    @Column(name = "DataAreaId")
    private String dataAreaId;

    @Column(name = "CompanyRefRecId")
    private Long companyRefRecId;

    @Column(name = "IntegrationName")
    private String integrationName;

    @Column(name = "IntegrationParameterId")
    private Long integrationParameterId;

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public Long getCategoryId() {
        return categoryId;
    }

    public void setCategoryId(Long categoryId) {
        this.categoryId = categoryId;
    }

    public String getExternalId() {
        return externalId;
    }

    public void setExternalId(String externalId) {
        this.externalId = externalId;
    }

    public String getName() {
        return Name;
    }

    public void setName(String name) {
        Name = name;
    }

    public String getExternalName() {
        return externalName;
    }

    public void setExternalName(String externalName) {
        this.externalName = externalName;
    }

    public String getDataAreaId() {
        return dataAreaId;
    }

    public void setDataAreaId(String dataAreaId) {
        this.dataAreaId = dataAreaId;
    }

    public Long getCompanyRefRecId() {
        return companyRefRecId;
    }

    public void setCompanyRefRecId(Long companyRefRecId) {
        this.companyRefRecId = companyRefRecId;
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
}
