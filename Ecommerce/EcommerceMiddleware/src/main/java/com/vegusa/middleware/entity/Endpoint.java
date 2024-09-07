package com.vegusa.middleware.entity;

import jakarta.persistence.*;
@Entity
@Table(name = "Endpoints")
public class Endpoint {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "id", columnDefinition = "int UNSIGNED not null")
    private Long id;

    @Column(name = "endpt_name", nullable = false, length = 50)
    private String endptName;

    @Column(name = "url", nullable = false, length = 250)
    private String url;

    @Column(name = "description", length = 250)
    private String description;
    @Column(nullable = false)
    private String integration_company;

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getEndptName() {
        return endptName;
    }

    public void setEndptName(String endptName) {
        this.endptName = endptName;
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
    public String getIntegrationCompany(){ return this.integration_company; }
    public void setIntegrationCompany(String integration_company){ this.integration_company = integration_company; }

}