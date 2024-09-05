package com.vegusa.middleware.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "interface")
public class InterfaceDS {
    @EmbeddedId
    private InterfaceDSId id;

    @Column(name = "Name", length = 100)
    private String name;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    private Company company;

    public InterfaceDSId getId() {
        return id;
    }

    public void setId(InterfaceDSId id) {
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