package com.vegusa.middleware.entity;

import jakarta.persistence.*;

import java.util.Date;

@Entity
@Table(name = "ScrapedImage")
public class ScrapedImage {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long recId;

    @Column(name = "ItemId", nullable = false, length = 100)
    private String itemId;

    @Column(name = "ItemName", length = 250)
    private String itemName;

    @Column(name = "PartNumberSearched", length = 100)
    private String partNumberSearched;

    @Column(name = "PartNumberFound", length = 100)
    private String partNumberFound;

    @Column(name = "ImageNumber", nullable = false)
    private Integer imageNumber;

    @Column(name = "BlobName", nullable = false, length = 50)
    private String blobName;

    @Column(name = "ImageUrl", nullable = false, length = 250)
    private String imageUrl;

    @Column(name = "Source", length = 20)
    private String source;

    @Column(name = "UpdatedAt")
    private Date updatedAt;

    @Column(name = "CreatedAt")
    private Date createdAt;

    @Column(name = "DataAreaId", nullable = false, length = 50)
    private String dataAreaId;

    @Column(name = "CompanyRefRecId", nullable = false)
    private Long companyRefRecId;

    //getters
    public Long getRecId() { return recId; }

    public String getItemId() {
        return itemId;
    }

    public String getItemName() {
        return itemName;
    }

    public String getPartNumberSearched() {
        return partNumberSearched;
    }

    public String getPartNumberFound() {
        return partNumberFound;
    }

    public Integer getImageNumber() {
        return imageNumber;
    }

    public String getBlobName() {
        return blobName;
    }

    public String getImageUrl() {
        return imageUrl;
    }

    public String getSource() {
        return source;
    }

    public Date getUpdatedAt(){ return updatedAt; }

    public Date getCreatedAt(){ return createdAt; }

    public String getDataAreaId() {
        return dataAreaId;
    }

    public Long getCompanyRefRecId(){ return companyRefRecId; }

    //setters
    public void setRecId(Long recId) {
        this.recId = recId;
    }
    public void setItemId(String itemId) {
        this.itemId = itemId;
    }
    public void setItemName(String itemName) {
        this.itemName = itemName;
    }
    public void setPartNumberSearched(String partNumberSearched) { this.partNumberSearched = partNumberSearched; }
    public void setPartNumberFound(String partNumberFound) {
        this.partNumberFound = partNumberFound;
    }
    public void setImageNumber(Integer imageNumber){ this.imageNumber = imageNumber; }
    public void setBlobName(String blobName) {
        this.blobName = blobName;
    }
    public void setImageUrl(String imageUrl) {
        this.imageUrl = imageUrl;
    }
    public void setSource(String source){ this.source = source; }
    public void setUpdatedAt(Date updatedAt){ this.updatedAt = updatedAt; }
    public void setCreatedAt(Date createdAt){ this.createdAt = createdAt; }
    public void setDataAreaId(String dataAreaId) {
        this.dataAreaId = dataAreaId;
    }
    public void setCompanyRefRecId(Long companyRefRecId){ this.companyRefRecId = companyRefRecId; }

}