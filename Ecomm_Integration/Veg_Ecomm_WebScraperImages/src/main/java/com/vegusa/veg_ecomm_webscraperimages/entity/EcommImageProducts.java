package com.vegusa.veg_ecomm_webscraperimages.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "veg_ecomm_scraped_images", schema = "dbo")
public class EcommImageProducts {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "id", columnDefinition = "int UNSIGNED not null")
    private Long id;
    @Column(name = "internal_product_id", nullable = false, length = 100)
    private String internalProductId;

    @Column(name = "product_name", length = 250)
    private String productName;
    @Column(name = "product_search_id", length = 100)
    private String productSearchId;
    @Column(name = "image_number", nullable = false)
    private Integer imageNumber;
    @Column(name = "blob_name", nullable = false, length = 50)
    private String blobName;
    @Column(name = "image_url", nullable = false, length = 250)
    private String imageUrl;
    @Column(name = "veg_business_unit", nullable = false, length = 50)
    private String vegBusinessUnit;

    //getters
    public String getInternalProductId(){ return this.internalProductId; }
    public String getProductName(){ return this.productName; }
    public String getProductSearchId(){ return this.productSearchId; }
    public Integer getImageNumber(){ return this.imageNumber; }
    public String getBlobName(){ return this.blobName; }
    public String getImageUrl(){ return this.imageUrl; }
    public String getVegBusinessUnit(){ return this.vegBusinessUnit; }

    //setters
    public void setInternalProductId(String internalProductId){ this.internalProductId = internalProductId; }
    public void setProductName(String productName){ this.productName = productName; }
    public void setProductSearchId(String productSearchId){ this.productSearchId = productSearchId; }
    public void setImageNumber(Integer imageNumber){this.imageNumber = imageNumber; }
    public void setBlobName(String blobName){ this.blobName = blobName; }
    public void setImageUrl(String imageUrl){ this.imageUrl = imageUrl; }
    public void setVegBusinessUnit(String vegBusinessUnit){ this.vegBusinessUnit = vegBusinessUnit; }

}
