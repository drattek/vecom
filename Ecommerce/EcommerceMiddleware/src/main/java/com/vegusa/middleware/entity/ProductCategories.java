package com.vegusa.middleware.entity;

import com.vegusa.middleware.constants.IntegrationType;
import jakarta.persistence.*;

import java.time.Instant;

@Entity
@Table(name = "productcategory")
public class ProductCategories {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long id;

    @Column(name = "CategoryRefRecId")
    private Long categoryRefRecId;

    @Column(name = "CategoryName")
    private String categoryName;

    @Column(name = "ItemId")
    private String itemId;

    @Column(name = "DataAreaId")
    private String dataAreaId;

    @Column(name = "CompanyRefRecId")
    private Long companyRefRecId;

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public Long getCategoryRefRecId() {
        return categoryRefRecId;
    }

    public void setCategoryRefRecId(Long categoryRefRecId) {
        this.categoryRefRecId = categoryRefRecId;
    }

    public String getCategoryName() {
        return categoryName;
    }

    public void setCategoryName(String categoryName) {
        this.categoryName = categoryName;
    }

    public String getItemId() {
        return itemId;
    }

    public void setItemId(String itemId) {
        this.itemId = itemId;
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
}
