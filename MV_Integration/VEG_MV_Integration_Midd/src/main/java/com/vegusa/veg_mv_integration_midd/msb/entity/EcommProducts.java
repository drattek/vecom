package com.vegusa.veg_mv_integration_midd.msb.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import org.hibernate.annotations.Immutable;
import org.hibernate.annotations.Nationalized;

/**
 * Mapping for DB view
 */
@Entity
@Immutable
@Table(name = "EcommProducts")
public class EcommProducts {
    @Id
    @Column(name = "RECID", nullable = false)
    private Long recid;

    @Nationalized
    @Column(name = "ItemId", nullable = false, length = 20)
    private String itemId;

    @Nationalized
    @Column(name = "InventBatchId", length = 30)
    private String inventBatchId;

    @Nationalized
    @Column(name = "InventColorId", length = 60)
    private String inventColorId;

    @Nationalized
    @Column(name = "InventLocationId", length = 10)
    private String inventLocationId;

    @Nationalized
    @Column(name = "InventSerialId", length = 30)
    private String inventSerialId;

    @Nationalized
    @Column(name = "InventSiteId", length = 10)
    private String inventSiteId;

    @Nationalized
    @Column(name = "InventSizeId", length = 60)
    private String inventSizeId;

    @Nationalized
    @Column(name = "InventStatusId", length = 10)
    private String inventStatusId;

    @Nationalized
    @Column(name = "InventStyleId", length = 60)
    private String inventStyleId;

    @Nationalized
    @Column(name = "wMSLocationId", length = 10)
    private String wMSLocationId;

    public Long getRecid() {
        return recid;
    }

    public String getItemId() {
        return itemId;
    }

    public String getInventBatchId() {
        return inventBatchId;
    }

    public String getInventColorId() {
        return inventColorId;
    }

    public String getInventLocationId() {
        return inventLocationId;
    }

    public String getInventSerialId() {
        return inventSerialId;
    }

    public String getInventSiteId() {
        return inventSiteId;
    }

    public String getInventSizeId() {
        return inventSizeId;
    }

    public String getInventStatusId() {
        return inventStatusId;
    }

    public String getInventStyleId() {
        return inventStyleId;
    }

    public String getWMSLocationId() {
        return wMSLocationId;
    }

    protected EcommProducts() {
    }
}