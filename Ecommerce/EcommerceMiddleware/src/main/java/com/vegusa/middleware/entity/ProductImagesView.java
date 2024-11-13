package com.vegusa.middleware.entity;

import jakarta.persistence.*;
import org.hibernate.annotations.Immutable;

import java.time.Instant;

/**
 * Mapping for DB view
 */
@Entity
@Immutable
@Table(name = "productimagesview")
public class ProductImagesView {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "ProductImagesRecId")
    private Long productImagesRecId;

    @Column(name = "SyncItemRecId", nullable = false)
    private Long syncItemRecId;

    @Column(name = "SyncItemResponseId", nullable = false, length = 100)
    private String syncItemResponseId;

    @Column(name = "ItemId", nullable = false, length = 100)
    private String itemId;

    @Column(name = "PartNumber", nullable = false, length = 100)
    private String partNumber;

    @Column(name = "ItemName", length = 250)
    private String itemName;

    @Column(name = "ImageUrl", nullable = false, length = 250)
    private String imageUrl;

    @Column(name = "BlobName", nullable = false, length = 50)
    private String blobName;

    @Column(name = "CreatedAt")
    private Instant createdAt;

    @Column(name = "UpdatedAt")
    private Instant updatedAt;

    @Column(name = "InterfaceId", nullable = false, length = 20)
    private String interfaceId;

    @Column(name = "DataAreaId", nullable = false, length = 20)
    private String dataAreaId;

    @Column(name = "ImageNumber", nullable = false)
    private Integer imageNumber;

    public Integer getImageNumber() {
        return imageNumber;
    }

    public Long getSyncItemRecId() {
        return syncItemRecId;
    }

    public String getSyncItemResponseId() {
        return syncItemResponseId;
    }

    public String getItemId() {
        return itemId;
    }

    public String getPartNumber() {
        return partNumber;
    }

    public String getItemName() {
        return itemName;
    }

    public String getImageUrl() {
        return imageUrl;
    }

    public String getBlobName() {
        return blobName;
    }

    public Instant getCreatedAt() {
        return createdAt;
    }

    public Instant getUpdatedAt() {
        return updatedAt;
    }

    public String getInterfaceId() {
        return interfaceId;
    }

    public String getDataAreaId() {
        return dataAreaId;
    }

    protected ProductImagesView() {
    }
}