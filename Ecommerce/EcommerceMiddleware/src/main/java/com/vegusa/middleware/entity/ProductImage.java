package com.vegusa.middleware.entity;

import jakarta.persistence.*;

import java.time.Instant;

@Entity
@Table(name = "productimages")
public class ProductImage {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId")
    private Long recId;

    @Column(name = "ItemId")
    private String itemId;

    @Column(name = "PartNumber")
    private String partNumber;

    @Column(name = "ItemName")
    private String itemName;

    @Column(name = "ImageUrl")
    private String imageUrl;

    @Column(name = "BlobName")
    private String blobName;

    @Column(name = "UpdatedAt")
    private Instant updatedAt;

    @Column(name = "CreatedAt")
    private Instant createdAt;

    @Column(name = "InterfaceId")
    private String interfaceId;

    @Column(name = "InterfaceRefRecId")
    private Long interfaceRefRecId;

    @Column(name = "DataAreaId")
    private String dataAreaId;

    @Column(name = "CompanyRefRecId")
    private Long companyRefRecId;

    @Column(name = "ImageNumber")
    private Long imageNumber;

    @Column(name = "IsActive")
    private Boolean isActive;

    @Column(name = "Priority")
    private Long priority;

    public Long getRecId() {
        return recId;
    }

    public void setRecId(Long recId) {
        this.recId = recId;
    }

    public String getItemId() {
        return itemId;
    }

    public void setItemId(String itemId) {
        this.itemId = itemId;
    }

    public String getPartNumber() {
        return partNumber;
    }

    public void setPartNumber(String partNumber) {
        this.partNumber = partNumber;
    }

    public String getItemName() {
        return itemName;
    }

    public void setItemName(String itemName) {
        this.itemName = itemName;
    }

    public String getImageUrl() {
        return imageUrl;
    }

    public void setImageUrl(String imageUrl) {
        this.imageUrl = imageUrl;
    }

    public String getBlobName() {
        return blobName;
    }

    public void setBlobName(String blobName) {
        this.blobName = blobName;
    }

    public Instant getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(Instant updatedAt) {
        this.updatedAt = updatedAt;
    }

    public Instant getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(Instant createdAt) {
        this.createdAt = createdAt;
    }

    public String getInterfaceId() {
        return interfaceId;
    }

    public void setInterfaceId(String interfaceId) {
        this.interfaceId = interfaceId;
    }

    public Long getInterfaceRefRecId() {
        return interfaceRefRecId;
    }

    public void setInterfaceRefRecId(Long interfaceRefRecId) {
        this.interfaceRefRecId = interfaceRefRecId;
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

    public Long getImageNumber() {
        return imageNumber;
    }

    public void setImageNumber(Long imageNumber) {
        this.imageNumber = imageNumber;
    }

    public Boolean getActive() {
        return isActive;
    }

    public void setActive(Boolean active) {
        isActive = active;
    }

    public Long getPriority() {
        return priority;
    }

    public void setPriority(Long priority) {
        this.priority = priority;
    }
}
