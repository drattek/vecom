package com.vegusa.middleware.integrations.jumpseller.entity;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import jakarta.persistence.*;

@Entity
@Table(name = "SyncCategory")
@JsonIgnoreProperties(ignoreUnknown = true)
public class SyncCategoryJumpseller {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId")
    private Long id;

    @Column(name = "ResponseId")
    private String responseId;

    @Column(name = "Name")
    private String name;

    @Column(name = "Branch")
    private String branch;

    @Column(name = "Description")
    private String description;

    @Column(name = "ResponseStatus")
    private String responseStatus;

    @Column(name = "DataAreaId")
    private String dataAreaId;

    @Column(name = "CompanyRefRecId")
    private Long companyRefRecId;

    @Column(name = "IntegrationCompany")
    private String integrationCompany;

    public SyncCategoryJumpseller() {}

    public SyncCategoryJumpseller(Long id, String responseId, String name, String branch, String description, String responseStatus, String dataAreaId, Long companyRefRecId, String integrationCompany) {
        this.id = id;
        this.responseId = responseId;
        this.name = name;
        this.branch = branch;
        this.description = description;
        this.responseStatus = responseStatus;
        this.dataAreaId = dataAreaId;
        this.companyRefRecId = companyRefRecId;
        this.integrationCompany = integrationCompany;
    }

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

    public String getResponseStatus() {
        return responseStatus;
    }

    public void setResponseStatus(String responseStatus) {
        this.responseStatus = responseStatus;
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

    public String getIntegrationCompany() {
        return integrationCompany;
    }

    public void setIntegrationCompany(String integrationsCompany) {
        this.integrationCompany = integrationsCompany;
    }
}
