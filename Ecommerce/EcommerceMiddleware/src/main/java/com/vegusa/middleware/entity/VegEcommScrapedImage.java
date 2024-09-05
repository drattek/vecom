package com.vegusa.middleware.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "veg_ecomm_scraped_images")
public class VegEcommScrapedImage {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "id", nullable = false)
    private Long id;

    @Column(name = "internal_product_id", nullable = false, length = 100)
    private String internalProductId;

    @Column(name = "product_search_id", length = 100)
    private String productSearchId;

    @Column(name = "image_url", nullable = false, length = 250)
    private String imageUrl;

    @Column(name = "veg_business_unit", nullable = false, length = 50)
    private String vegBusinessUnit;

    @Column(name = "blob_name", nullable = false, length = 50)
    private String blobName;

    @Column(name = "image_number", nullable = false)
    private Integer imageNumber;

    @Column(name = "product_name", length = 250)
    private String productName;

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getInternalProductId() {
        return internalProductId;
    }

    public void setInternalProductId(String internalProductId) {
        this.internalProductId = internalProductId;
    }

    public String getProductSearchId() {
        return productSearchId;
    }

    public void setProductSearchId(String productSearchId) {
        this.productSearchId = productSearchId;
    }

    public String getImageUrl() {
        return imageUrl;
    }

    public void setImageUrl(String imageUrl) {
        this.imageUrl = imageUrl;
    }

    public String getVegBusinessUnit() {
        return vegBusinessUnit;
    }

    public void setVegBusinessUnit(String vegBusinessUnit) {
        this.vegBusinessUnit = vegBusinessUnit;
    }

    public String getBlobName() {
        return blobName;
    }

    public void setBlobName(String blobName) {
        this.blobName = blobName;
    }

    public Integer getImageNumber() {
        return imageNumber;
    }

    public void setImageNumber(Integer imageNumber) {
        this.imageNumber = imageNumber;
    }

    public String getProductName() {
        return productName;
    }

    public void setProductName(String productName) {
        this.productName = productName;
    }

}