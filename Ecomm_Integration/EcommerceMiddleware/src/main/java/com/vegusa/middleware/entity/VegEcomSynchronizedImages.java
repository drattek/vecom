package com.vegusa.middleware.entity;

import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.persistence.*;

import java.util.Date;

@Entity
@Table(name = "veg_ecomm_synchronized_images")
public class VegEcomSynchronizedImages {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "id", nullable = false)
    private Long id;

    @Column(name = "product_id", length = 100)
    @JsonProperty("ProductId")
    private String productId;

    @Column(name = "url", length = 250)
    private String url;

    @Column(name = "id_ecomm", length = 100)
    @JsonProperty("_id")
    private String idEcomm;

    @Column(name = "status", length = 50)
    private String status;

    @Column(name = "provider", length = 50)
    private String provider;

    @Column(name = "storage_url", length = 250)
    @JsonProperty("storageUrl")
    private String storageUrl;

    @Column(name = "file_key", length = 250)
    @JsonProperty("fileKey")
    private String fileKey;

    @Column(name = "file_size", length = 50)
    @JsonProperty("fileSize")
    private String fileSize;

    @Column(name = "file_type", length = 50)
    @JsonProperty("fileType")
    private String fileType;

    @Column(name = "original_server_path", length = 250)
    @JsonProperty("originalServerPath")
    private String originalServerPath;

    @Column(name = "original_file_name", length = 250)
    @JsonProperty("originalFileName")
    private String originalFileName;

    @Column(name = "position")
    private Integer position;

    @Column(name = "merchant_id", length = 100)
    @JsonProperty("MerchantId")
    private String merchantId;

    @Column(name = "created_by_id", length = 100)
    @JsonProperty("CreatedById")
    private String createdById;

    @Column(name = "updated_by_id", length = 100)
    @JsonProperty("UpdatedById")
    private String updatedById;

    @Column(name = "product_picture_set_id", length = 100)
    @JsonProperty("ProductPictureSetId")
    private String productPictureSetId;

    @Column(name = "updated_at")
    @JsonProperty("updatedAt")
    private Date updatedAt;

    @Column(name = "created_at")
    @JsonProperty("createdAt")
    private Date createdAt;

    @Column(name = "product_id_error", length = 100)
    @JsonProperty("productId")
    private String productIdError;

    @Column(name = "image_error", length = 250)
    @JsonProperty("image")
    private String imageError;

    @Column(name = "error", length = 500)
    private String error;
    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getProductId() {
        return productId;
    }

    public void setProductId(String productId) {
        this.productId = productId;
    }

    public String getUrl() {
        return url;
    }

    public void setUrl(String url) {
        this.url = url;
    }

    public String getIdEcomm() {
        return idEcomm;
    }

    public void setIdEcomm(String idEcomm) {
        this.idEcomm = idEcomm;
    }

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
    }

    public String getProvider() {
        return provider;
    }

    public void setProvider(String provider) {
        this.provider = provider;
    }

    public String getStorageUrl() {
        return storageUrl;
    }

    public void setStorageUrl(String storageUrl) {
        this.storageUrl = storageUrl;
    }

    public String getFileKey() {
        return fileKey;
    }

    public void setFileKey(String fileKey) {
        this.fileKey = fileKey;
    }

    public String getFileSize() {
        return fileSize;
    }

    public void setFileSize(String fileSize) {
        this.fileSize = fileSize;
    }

    public String getFileType() {
        return fileType;
    }

    public void setFileType(String fileType) {
        this.fileType = fileType;
    }

    public String getOriginalServerPath() {
        return originalServerPath;
    }

    public void setOriginalServerPath(String originalServerPath) {
        this.originalServerPath = originalServerPath;
    }

    public String getOriginalFileName() {
        return originalFileName;
    }

    public void setOriginalFileName(String originalFileName) {
        this.originalFileName = originalFileName;
    }

    public Integer getPosition() {
        return position;
    }

    public void setPosition(Integer position) {
        this.position = position;
    }

    public String getMerchantId() {
        return merchantId;
    }

    public void setMerchantId(String merchantId) {
        this.merchantId = merchantId;
    }

    public String getCreatedById() {
        return createdById;
    }

    public void setCreatedById(String createdById) {
        this.createdById = createdById;
    }

    public String getUpdatedById() {
        return updatedById;
    }

    public void setUpdatedById(String updatedById) {
        this.updatedById = updatedById;
    }

    public String getProductPictureSetId() {
        return productPictureSetId;
    }

    public void setProductPictureSetId(String productPictureSetId) {
        this.productPictureSetId = productPictureSetId;
    }

    public Date getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(Date updatedAt) {
        this.updatedAt = updatedAt;
    }

    public Date getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(Date createdAt) {
        this.createdAt = createdAt;
    }

    public String getImageError() {
        return imageError;
    }

    public void setImageError(String imageError) {
        this.imageError = imageError;
    }

    public String getError() {
        return error;
    }

    public void setError(String error) {
        this.error = error;
    }

    public String getProductIdError() {
        return productIdError;
    }

    public void setProductIdError(String productIdError) {
        this.productIdError = productIdError;
    }

}