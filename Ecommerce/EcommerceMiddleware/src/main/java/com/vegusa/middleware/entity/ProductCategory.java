package com.vegusa.middleware.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "productcategory")
public class ProductCategory {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long id;

    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumns({
            @JoinColumn(name = "CategoryRefRecId", referencedColumnName = "RecId"),
            @JoinColumn(name = "CategoryName", referencedColumnName = "Name")
    })
    private Category category;

    @Column(name = "ItemId", nullable = false, length = 50)
    private String itemId;

    @Column(name = "DataAreaId", nullable = false, length = 20)
    private String dataAreaId;

    @Column(name = "CompanyRefRecId", columnDefinition = "int UNSIGNED not null")
    private Long companyRefRecId;

    public Long getCompanyRefRecId() {
        return companyRefRecId;
    }

    public void setCompanyRefRecId(Long companyRefRecId) {
        this.companyRefRecId = companyRefRecId;
    }

    public String getDataAreaId() {
        return dataAreaId;
    }

    public void setDataAreaId(String dataAreaId) {
        this.dataAreaId = dataAreaId;
    }

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public Category getCategory() {
        return category;
    }

    public void setCategory(Category category) {
        this.category = category;
    }

    public String getItemId() {
        return itemId;
    }

    public void setItemId(String itemId) {
        this.itemId = itemId;
    }

}