package com.vegusa.middleware.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Table;
import jakarta.persistence.EmbeddedId;

@Entity
@Table(name = "company")
public class Company {
    @EmbeddedId
    private CompanyId id;

    @Column(name = "Name", length = 100)
    private String name;

    public CompanyId getId() {
        return id;
    }

    public void setId(CompanyId id) {
        this.id = id;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

}