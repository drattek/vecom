package com.vegusa.veg_mv_integration_midd.veg_middleware.entity;

import jakarta.persistence.*;

import java.time.Instant;

@Entity
@Table(name = "veg_ecomm_scraped_additional_info")
public class ProductAdditionalInfo {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "id", nullable = false)
    private Long id;

    @Column(name = "internal_product_id", nullable = false, length = 100)
    private String internalProductId;

    @Column(name = "product_name", length = 250)
    private String productName;

    @Column(name = "product_search_id", length = 100)
    private String productSearchId;

    @Column(name = "short_description", length = 250)
    private String shortDescription;

    @Column(name = "weight", length = 20)
    private String weight;

    @Column(name = "unit_of_measure", length = 20)
    private String unitOfMeasure;

    @Lob
    @Column(name = "cross_reference")
    private String crossReference;

    @Column(name = "download_portal", length = 20)
    private String downloadPortal;

    @Column(name = "updated_at")
    private Instant updatedAt;

    @Column(name = "created_at")
    private Instant createdAt;

    @Column(name = "veg_business_unit", nullable = false, length = 50)
    private String vegBusinessUnit;

    @Column(name = "validation_reason", length = 20)
    private String validationReason;

    @Column(name = "contract_price", length = 20)
    private String contractPrice;

    @Column(name = "image_url", length = 100)
    private String imageUrl;

    @Column(name = "oem_id", length = 20)
    private String oemId;

    @Column(name = "uca_id", length = 20)
    private String ucaId;

    @Column(name = "fleet_list_price", length = 20)
    private String fleetListPrice;

    @Column(name = "dealer_net_price", length = 20)
    private String dealerNetPrice;

    @Column(name = "company", length = 20)
    private String company;

    @Column(name = "marketing_description", length = 250)
    private String marketingDescription;

    @Column(name = "item_category", length = 20)
    private String itemCategory;

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

    public String getProductName() {
        return productName;
    }

    public void setProductName(String productName) {
        this.productName = productName;
    }

    public String getProductSearchId() {
        return productSearchId;
    }

    public void setProductSearchId(String productSearchId) {
        this.productSearchId = productSearchId;
    }

    public String getShortDescription() {
        return shortDescription;
    }

    public void setShortDescription(String shortDescription) {
        this.shortDescription = shortDescription;
    }

    public String getWeight() {
        return weight;
    }

    public void setWeight(String weight) {
        this.weight = weight;
    }

    public String getUnitOfMeasure() {
        return unitOfMeasure;
    }

    public void setUnitOfMeasure(String unitOfMeasure) {
        this.unitOfMeasure = unitOfMeasure;
    }

    public String getCrossReference() {
        return crossReference;
    }

    public void setCrossReference(String crossReference) {
        this.crossReference = crossReference;
    }

    public String getDownloadPortal() {
        return downloadPortal;
    }

    public void setDownloadPortal(String downloadPortal) {
        this.downloadPortal = downloadPortal;
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

    public String getVegBusinessUnit() {
        return vegBusinessUnit;
    }

    public void setVegBusinessUnit(String vegBusinessUnit) {
        this.vegBusinessUnit = vegBusinessUnit;
    }

    public String getValidationReason() {
        return validationReason;
    }

    public void setValidationReason(String validationReason) {
        this.validationReason = validationReason;
    }

    public String getContractPrice() {
        return contractPrice;
    }

    public void setContractPrice(String contractPrice) {
        this.contractPrice = contractPrice;
    }

    public String getImageUrl() {
        return imageUrl;
    }

    public void setImageUrl(String imageUrl) {
        this.imageUrl = imageUrl;
    }

    public String getOemId() {
        return oemId;
    }

    public void setOemId(String oemId) {
        this.oemId = oemId;
    }

    public String getUcaId() {
        return ucaId;
    }

    public void setUcaId(String ucaId) {
        this.ucaId = ucaId;
    }

    public String getFleetListPrice() {
        return fleetListPrice;
    }

    public void setFleetListPrice(String fleetListPrice) {
        this.fleetListPrice = fleetListPrice;
    }

    public String getDealerNetPrice() {
        return dealerNetPrice;
    }

    public void setDealerNetPrice(String dealerNetPrice) {
        this.dealerNetPrice = dealerNetPrice;
    }

    public String getCompany() {
        return company;
    }

    public void setCompany(String company) {
        this.company = company;
    }

    public String getMarketingDescription() {
        return marketingDescription;
    }

    public void setMarketingDescription(String marketingDescription) {
        this.marketingDescription = marketingDescription;
    }

    public String getItemCategory() {
        return itemCategory;
    }

    public void setItemCategory(String itemCategory) {
        this.itemCategory = itemCategory;
    }

}