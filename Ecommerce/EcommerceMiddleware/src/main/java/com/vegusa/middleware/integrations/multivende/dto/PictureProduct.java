package com.vegusa.middleware.integrations.multivende.dto;

public class PictureProduct {
    private String _id;
    private String url;
    private String provider;
    private String title;
    private String alt;
    private String description;
    private String storageUrl;
    private String storagePath;
    private String fileKey;
    private String originalServerPath;
    private String originalFileName;
    private String fileName;
    private String fileType;
    private Long fileSize;
    private Integer position;
    private String status;
    private String productPictureSetId;
    private String productId;
    private String merchantId;
    private PictureSet productPictureSet;
    private String productVersionId;

    public PictureProduct() {}

    public PictureProduct(String _id, String url, String provider, String title, String alt, String description, String storageUrl, String storagePath, String fileKey, String originalServerPath, String originalFileName, String fileName, String fileType, Long fileSize, Integer position, String status, String productPictureSetId, String productId, String merchantId, PictureSet productPictureSet, String productVersionId) {
        this._id = _id;
        this.url = url;
        this.provider = provider;
        this.title = title;
        this.alt = alt;
        this.description = description;
        this.storageUrl = storageUrl;
        this.storagePath = storagePath;
        this.fileKey = fileKey;
        this.originalServerPath = originalServerPath;
        this.originalFileName = originalFileName;
        this.fileName = fileName;
        this.fileType = fileType;
        this.fileSize = fileSize;
        this.position = position;
        this.status = status;
        this.productPictureSetId = productPictureSetId;
        this.productId = productId;
        this.merchantId = merchantId;
        this.productPictureSet = productPictureSet;
        this.productVersionId = productVersionId;
    }

    public String get_id() {
        return _id;
    }

    public void set_id(String _id) {
        this._id = _id;
    }

    public String getUrl() {
        return url;
    }

    public void setUrl(String url) {
        this.url = url;
    }

    public String getProvider() {
        return provider;
    }

    public void setProvider(String provider) {
        this.provider = provider;
    }

    public String getTitle() {
        return title;
    }

    public void setTitle(String title) {
        this.title = title;
    }

    public String getAlt() {
        return alt;
    }

    public void setAlt(String alt) {
        this.alt = alt;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getStorageUrl() {
        return storageUrl;
    }

    public void setStorageUrl(String storageUrl) {
        this.storageUrl = storageUrl;
    }

    public String getStoragePath() {
        return storagePath;
    }

    public void setStoragePath(String storagePath) {
        this.storagePath = storagePath;
    }

    public String getFileKey() {
        return fileKey;
    }

    public void setFileKey(String fileKey) {
        this.fileKey = fileKey;
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

    public String getFileName() {
        return fileName;
    }

    public void setFileName(String fileName) {
        this.fileName = fileName;
    }

    public String getFileType() {
        return fileType;
    }

    public void setFileType(String fileType) {
        this.fileType = fileType;
    }

    public Long getFileSize() {
        return fileSize;
    }

    public void setFileSize(Long fileSize) {
        this.fileSize = fileSize;
    }

    public Integer getPosition() {
        return position;
    }

    public void setPosition(Integer position) {
        this.position = position;
    }

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
    }

    public String getProductPictureSetId() {
        return productPictureSetId;
    }

    public void setProductPictureSetId(String productPictureSetId) {
        this.productPictureSetId = productPictureSetId;
    }

    public String getProductId() {
        return productId;
    }

    public void setProductId(String productId) {
        this.productId = productId;
    }

    public String getMerchantId() {
        return merchantId;
    }

    public void setMerchantId(String merchantId) {
        this.merchantId = merchantId;
    }

    public PictureSet getProductPictureSet() {
        return productPictureSet;
    }

    public void setProductPictureSet(PictureSet productPictureSet) {
        this.productPictureSet = productPictureSet;
    }

    public String getProductVersionId() {
        return productVersionId;
    }

    public void setProductVersionId(String productVersionId) {
        this.productVersionId = productVersionId;
    }
}
