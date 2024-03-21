package com.vegusa.veg_mv_integration_midd.msb.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import org.hibernate.annotations.Immutable;
import org.hibernate.annotations.Nationalized;

import java.time.Instant;

/**
 * Mapping for DB view
 */
@Entity
@Immutable
public class EcoResProduct {
    @Id
    @Column(name = "RECID", nullable = false)
    private Long recid;

    @Column(name = "\"$FileName\"", length = 100)
    private String $FileName;

    @Column(name = "_SysRowId", nullable = false)
    private Long sysrowid;

    @Nationalized
    @Column(name = "LSN", nullable = false, length = 60)
    private String lsn;

    @Column(name = "LastProcessedChange_DateTime", nullable = false)
    private Instant lastprocessedchangeDatetime;

    @Column(name = "DataLakeModified_DateTime", nullable = false)
    private Instant datalakemodifiedDatetime;

    @Nationalized
    @Column(name = "DisplayProductNumber", nullable = false, length = 240)
    private String displayProductNumber;

    @Column(name = "InstanceRelationType")
    private Long instanceRelationType;

    @Column(name = "PdsCWProduct")
    private Integer pdsCWProduct;

    @Column(name = "ProductType", nullable = false)
    private Integer productType;

    @Nationalized
    @Column(name = "SearchName", length = 20)
    private String searchName;

    @Column(name = "ServiceType")
    private Integer serviceType;

    @Nationalized
    @Column(name = "EngChgProductOwnerId", length = 90)
    private String engChgProductOwnerId;

    @Column(name = "EngChgProductCategoryDetails")
    private Long engChgProductCategoryDetails;

    @Column(name = "EngChgProductReleasePolicy")
    private Long engChgProductReleasePolicy;

    @Column(name = "EngChgProductReadinessPolicy")
    private Long engChgProductReadinessPolicy;

    @Column(name = "PRODUCTMASTER", nullable = false)
    private Long productmaster;

    @Column(name = "RETAITOTALWEIGHT", nullable = false)
    private Integer retaitotalweight;

    @Nationalized
    @Column(name = "RETAILCOLORGROUPID", nullable = false, length = 1000)
    private String retailcolorgroupid;

    @Nationalized
    @Column(name = "RETAILSIZEGROUPID", nullable = false, length = 1000)
    private String retailsizegroupid;

    @Nationalized
    @Column(name = "RETAILSTYLEGROUPID", nullable = false, length = 1000)
    private String retailstylegroupid;

    @Column(name = "VARIANTCONFIGURATIONTECHNOLOGY", nullable = false)
    private Integer variantconfigurationtechnology;

    @Column(name = "ISPRODUCTVARIANTUNITCONVERSIONENABLED", nullable = false)
    private Integer isproductvariantunitconversionenabled;

    @Column(name = "PARTITION", nullable = false)
    private Long partition;

    @Column(name = "RECVERSION", nullable = false)
    private Integer recversion;

    @Nationalized
    @Column(name = "MODIFIEDBY", nullable = false, length = 20)
    private String modifiedby;

    @Column(name = "RELATIONTYPE", nullable = false)
    private Long relationtype;

    public Long getRecid() {
        return recid;
    }

    public String get$FileName() {
        return $FileName;
    }

    public Long getSysrowid() {
        return sysrowid;
    }

    public String getLsn() {
        return lsn;
    }

    public Instant getLastprocessedchangeDatetime() {
        return lastprocessedchangeDatetime;
    }

    public Instant getDatalakemodifiedDatetime() {
        return datalakemodifiedDatetime;
    }

    public String getDisplayProductNumber() {
        return displayProductNumber;
    }

    public Long getInstanceRelationType() {
        return instanceRelationType;
    }

    public Integer getPdsCWProduct() {
        return pdsCWProduct;
    }

    public Integer getProductType() {
        return productType;
    }

    public String getSearchName() {
        return searchName;
    }

    public Integer getServiceType() {
        return serviceType;
    }

    public String getEngChgProductOwnerId() {
        return engChgProductOwnerId;
    }

    public Long getEngChgProductCategoryDetails() {
        return engChgProductCategoryDetails;
    }

    public Long getEngChgProductReleasePolicy() {
        return engChgProductReleasePolicy;
    }

    public Long getEngChgProductReadinessPolicy() {
        return engChgProductReadinessPolicy;
    }

    public Long getProductmaster() {
        return productmaster;
    }

    public Integer getRetaitotalweight() {
        return retaitotalweight;
    }

    public String getRetailcolorgroupid() {
        return retailcolorgroupid;
    }

    public String getRetailsizegroupid() {
        return retailsizegroupid;
    }

    public String getRetailstylegroupid() {
        return retailstylegroupid;
    }

    public Integer getVariantconfigurationtechnology() {
        return variantconfigurationtechnology;
    }

    public Integer getIsproductvariantunitconversionenabled() {
        return isproductvariantunitconversionenabled;
    }

    public Long getPartition() {
        return partition;
    }

    public Integer getRecversion() {
        return recversion;
    }

    public String getModifiedby() {
        return modifiedby;
    }

    public Long getRelationtype() {
        return relationtype;
    }

    protected EcoResProduct() {
    }
}