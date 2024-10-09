package com.vegusa.middleware.entity;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.persistence.*;

import java.util.Date;

@Entity
@Table(name = "SyncImage")
@JsonIgnoreProperties(ignoreUnknown = true)
public class SyncImage {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId")
    private Long recId;

    @Column(name = "ProductId")
    @JsonProperty("ProductId")
    private String productId;

    @Column(name = "Url")
    @JsonProperty("url")
    private String url;

    @Column(name = "ResponseId")
    @JsonProperty("_id")
    private String responseId;

    @Column(name = "Status")
    @JsonProperty("status")
    private String status;

    @Column(name = "Provider")
    @JsonProperty("provider")
    private String provider;

    @Column(name = "StorageUrl")
    @JsonProperty("storageUrl")
    private String storageUrl;

    @Column(name = "FileKey")
    @JsonProperty("fileKey")
    private String fileKey;

    @Column(name = "FileSize")
    @JsonProperty("fileSize")
    private String fileSize;

    @Column(name = "FileType")
    @JsonProperty("fileType")
    private String fileType;

    @Column(name = "OriginalServerPath")
    @JsonProperty("originalServerPath")
    private String originalServerPath;

    @Column(name = "OriginalFileName")
    @JsonProperty("originalFileName")
    private String originalFileName;

    @Column(name = "Position")
    @JsonProperty("position")
    private int position;

    @Column(name = "MerchantId")
    @JsonProperty("MerchantId")
    private String merchantId;

    @Column(name = "CreatedById")
    @JsonProperty("CreatedById")
    private String createdById;

    @Column(name = "UpdatedById")
    @JsonProperty("UpdatedById")
    private String updatedById;

    @Column(name = "ProductPictureSetId")
    @JsonProperty("ProductPictureSetId")
    private String productPictureSetId;

    @Column(name = "UpdatedAt")
    @JsonProperty("updatedAt")
    private Date updatedAt;

    @Column(name = "CreatedAt")
    @JsonProperty("createdAt")
    private Date createdAt;

    @Column(name = "ProductIdError")
    @JsonProperty("productId")
    private String productIdError;

    @Column(name = "ImageError")
    @JsonProperty("image")
    private String imageError;

    @Column(name = "Error")
    private String error;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    private Company company;

    //getters
    public String getProductId(){ return productId; }
    public String getUrl(){ return url; }
    public String getResponseId(){ return responseId; }
    public String getStatus(){ return status; }
    public String getProvider(){ return provider; }
    public String getStorageUrl(){ return storageUrl;}
    public String getFileKey(){ return fileKey; }
    public String getFileSize(){ return fileSize; }
    public String getFileType(){ return fileType; }
    public String getOriginalServerPath(){ return originalServerPath; }
    public String getOriginalFileName(){ return originalFileName; }
    public int getPosition(){ return position; }
    public String getMerchantId(){ return merchantId; }
    public String getCreatedById(){ return createdById; }
    public String getUpdatedById(){ return updatedById; }
    public String getProductPictureSetId(){ return productPictureSetId; }
    public Date getUpdatedAt(){ return updatedAt; }
    public Date getCreatedAt(){ return createdAt; }
    public String getProductIdError(){ return productIdError; }
    public String getImageError(){ return imageError; }
    public String getError(){ return error; }
    public Company getCompany() { return company; }

    //setters
    public void setCompany(Company company) {
        this.company = company;
    }
}
