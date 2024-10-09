package com.vegusa.middleware.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "interface")
public class Interface {
    @EmbeddedId
    private InterfaceId id;

    @Column(name = "Name", length = 100)
    private String name;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    private Company company;

    public InterfaceId getId() {
        return id;
    }

    public void setId(InterfaceId id) {
        this.id = id;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public Company getCompany() {
        return company;
    }

    public void setCompany(Company company) {
        this.company = company;
    }

}