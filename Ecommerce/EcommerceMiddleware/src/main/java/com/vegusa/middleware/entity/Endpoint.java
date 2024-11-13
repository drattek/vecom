package com.vegusa.middleware.entity;

import jakarta.persistence.*;
@Entity
@Table(name = "Endpoint")
public class Endpoint {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", columnDefinition = "int UNSIGNED not null")
    private Long recId;

    @Column(name = "Name", nullable = false, length = 50)
    private String name;

    @Column(name = "Url", nullable = false, length = 250)
    private String url;

    @Column(name = "Description", length = 250)
    private String description;

    @Column(name="IntegrationCompany", nullable = false)
    private String integrationCompany;

    public Long getRecId() {
        return recId;
    }

    public void setRecId(Long recId) {
        this.recId = recId;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getUrl() {
        return url;
    }

    public void setUrl(String url) {
        this.url = url;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getIntegrationCompany(){ return this.integrationCompany; }

    public void setIntegrationCompany(String integration_company){ this.integrationCompany = integration_company; }

}