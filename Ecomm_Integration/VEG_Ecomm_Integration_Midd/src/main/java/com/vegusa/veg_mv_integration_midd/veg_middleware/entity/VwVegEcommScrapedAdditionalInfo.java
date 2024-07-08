package com.vegusa.veg_mv_integration_midd.veg_middleware.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import org.hibernate.annotations.Immutable;

import java.time.Instant;

/**
 * Mapping for DB view
 */
@Entity
@Immutable
@Table(name = "vw_veg_ecomm_scraped_additional_info")
public class VwVegEcommScrapedAdditionalInfo {
    @Id
    @Column(name = "id", nullable = false)
    private Long id;

    @Column(name = "id_mv", nullable = false, length = 100)
    private String idMv;

    @Column(name = "internal_product_id", nullable = false, length = 100)
    private String internalProductId;

    @Column(name = "product_name", length = 250)
    private String productName;

    @Column(name = "product_search_id", length = 100)
    private String productSearchId;

    @Column(name = "short_description", length = 150)
    private String shortDescription;

    @Column(name = "weight", length = 20)
    private String weight;

    @Column(name = "unit_of_measure", length = 20)
    private String unitOfMeasure;

    @Column(name = "cross_reference", length = 10000)
    private String crossReference;

    @Column(name = "download_portal", length = 20)
    private String downloadPortal;

    @Column(name = "updated_at")
    private Instant updatedAt;

    @Column(name = "created_at")
    private Instant createdAt;

    @Column(name = "veg_business_unit", nullable = false, length = 50)
    private String vegBusinessUnit;

    public Long getId() {
        return id;
    }

    public String getIdMv() {
        return idMv;
    }

    public String getInternalProductId() {
        return internalProductId;
    }

    public String getProductName() {
        return productName;
    }

    public String getProductSearchId() {
        return productSearchId;
    }

    public String getShortDescription() {
        return shortDescription;
    }

    public String getWeight() {
        return weight;
    }

    public String getUnitOfMeasure() {
        return unitOfMeasure;
    }

    public String getCrossReference() {
        return crossReference;
    }

    public String getDownloadPortal() {
        return downloadPortal;
    }

    public Instant getUpdatedAt() {
        return updatedAt;
    }

    public Instant getCreatedAt() {
        return createdAt;
    }

    public String getVegBusinessUnit() {
        return vegBusinessUnit;
    }

    protected VwVegEcommScrapedAdditionalInfo() {
    }
}