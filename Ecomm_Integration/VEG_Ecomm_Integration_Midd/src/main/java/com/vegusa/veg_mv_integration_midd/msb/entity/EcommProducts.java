package com.vegusa.veg_mv_integration_midd.msb.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import org.hibernate.annotations.Immutable;
import org.hibernate.annotations.Nationalized;

import java.math.BigDecimal;
import java.time.Instant;

/**
 * Mapping for DB view
 */
@Entity
@Immutable
@Table(name = "EcommProducts", schema = "dbo")
public class EcommProducts {
    @Id
    @Column(name = "InventSum_RECID", nullable = false)
    private Long inventsumRecid;

    @Column(name = "Arrived", precision = 32, scale = 16)
    private BigDecimal arrived;

    @Column(name = "AvailOrdered", precision = 32, scale = 16)
    private BigDecimal availOrdered;

    @Column(name = "AvailPhysical", precision = 32, scale = 16)
    private BigDecimal availPhysical;

    @Column(name = "Closed")
    private Integer closed;

    @Column(name = "ClosedQty")
    private Integer closedQty;

    @Column(name = "Deducted", precision = 32, scale = 16)
    private BigDecimal deducted;

    @Nationalized
    @Column(name = "InventDimId", nullable = false, length = 20)
    private String inventDimId;

    @Nationalized
    @Column(name = "ItemId", nullable = false, length = 20)
    private String itemId;

    @Column(name = "OnOrder", precision = 32, scale = 16)
    private BigDecimal onOrder;

    @Column(name = "Ordered", precision = 32, scale = 16)
    private BigDecimal ordered;

    @Column(name = "PhysicalInvent", precision = 32, scale = 16)
    private BigDecimal physicalInvent;

    @Column(name = "PhysicalValue", precision = 32, scale = 16)
    private BigDecimal physicalValue;

    @Column(name = "Picked", precision = 32, scale = 16)
    private BigDecimal picked;

    @Column(name = "PostedQty", precision = 32, scale = 16)
    private BigDecimal postedQty;

    @Column(name = "PostedValue", precision = 32, scale = 16)
    private BigDecimal postedValue;

    @Column(name = "Received", precision = 32, scale = 16)
    private BigDecimal received;

    @Column(name = "Registered", precision = 32, scale = 16)
    private BigDecimal registered;

    @Column(name = "ReservOrdered", precision = 32, scale = 16)
    private BigDecimal reservOrdered;

    @Column(name = "ReservPhysical", precision = 32, scale = 16)
    private BigDecimal reservPhysical;

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
    @Column(name = "InventVersionId", length = 10)
    private String inventVersionId;

    @Nationalized
    @Column(name = "wMSLocationId", length = 10)
    private String wMSLocationId;

    @Nationalized
    @Column(name = "WMSPALLETID", nullable = false, length = 1000)
    private String wmspalletid;

    @Column(name = "InventSum_MODIFIEDDATETIME", nullable = false)
    private Instant inventsumModifieddatetime;

    @Column(name = "InventTable_RECID", nullable = false)
    private Long inventtableRecid;

    @Nationalized
    @Column(name = "BOMUnitId", length = 10)
    private String bOMUnitId;

    @Column(name = "Density", precision = 32, scale = 16)
    private BigDecimal density;

    @Column(name = "Depth", precision = 32, scale = 16)
    private BigDecimal depth;

    @Column(name = "grossDepth", precision = 32, scale = 16)
    private BigDecimal grossDepth;

    @Column(name = "grossHeight", precision = 32, scale = 16)
    private BigDecimal grossHeight;

    @Column(name = "grossWidth", precision = 32, scale = 16)
    private BigDecimal grossWidth;

    @Column(name = "Height", precision = 32, scale = 16)
    private BigDecimal height;

    @Nationalized
    @Column(name = "NameAlias", length = 20)
    private String nameAlias;

    @Column(name = "NetWeight", precision = 32, scale = 16)
    private BigDecimal netWeight;

    @Nationalized
    @Column(name = "PrimaryVendorId", length = 20)
    private String primaryVendorId;

    @Column(name = "Product", nullable = false)
    private Long product;

    @Nationalized
    @Column(name = "projCategoryId", length = 30)
    private String projCategoryId;

    @Nationalized
    @Column(name = "SATCodeId_MX", length = 10)
    private String satcodeidMx;

    @Column(name = "InventTable_MODIFIEDDATETIME", nullable = false)
    private Instant inventtableModifieddatetime;

    @Column(name = "EcoResProduct_RECID", nullable = false)
    private Long ecoresproductRecid;

    @Nationalized
    @Column(name = "DisplayProductNumber", nullable = false, length = 240)
    private String displayProductNumber;

    @Nationalized
    @Column(name = "SearchName", length = 20)
    private String searchName;

    @Column(name = "PRODUCTMASTER", nullable = false)
    private Long productmaster;

    @Column(name = "EcoResProductTranslation_RECID")
    private Long ecoresproducttranslationRecid;

    @Nationalized
    @Column(name = "Description", length = 1000)
    private String description;

    @Nationalized
    @Column(name = "Name", length = 60)
    private String name;

    @Column(name = "InventItemGroupItem_RECID", nullable = false)
    private Long inventitemgroupitemRecid;

    @Nationalized
    @Column(name = "ItemGroupId", length = 10)
    private String itemGroupId;

    public Long getInventsumRecid() {
        return inventsumRecid;
    }

    public BigDecimal getArrived() {
        return arrived;
    }

    public BigDecimal getAvailOrdered() {
        return availOrdered;
    }

    public BigDecimal getAvailPhysical() {
        return availPhysical;
    }

    public Integer getClosed() {
        return closed;
    }

    public Integer getClosedQty() {
        return closedQty;
    }

    public BigDecimal getDeducted() {
        return deducted;
    }

    public String getInventDimId() {
        return inventDimId;
    }

    public String getItemId() {
        return itemId;
    }

    public BigDecimal getOnOrder() {
        return onOrder;
    }

    public BigDecimal getOrdered() {
        return ordered;
    }

    public BigDecimal getPhysicalInvent() {
        return physicalInvent;
    }

    public BigDecimal getPhysicalValue() {
        return physicalValue;
    }

    public BigDecimal getPicked() {
        return picked;
    }

    public BigDecimal getPostedQty() {
        return postedQty;
    }

    public BigDecimal getPostedValue() {
        return postedValue;
    }

    public BigDecimal getReceived() {
        return received;
    }

    public BigDecimal getRegistered() {
        return registered;
    }

    public BigDecimal getReservOrdered() {
        return reservOrdered;
    }

    public BigDecimal getReservPhysical() {
        return reservPhysical;
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

    public String getInventVersionId() {
        return inventVersionId;
    }

    public String getWMSLocationId() {
        return wMSLocationId;
    }

    public String getWmspalletid() {
        return wmspalletid;
    }

    public Instant getInventsumModifieddatetime() {
        return inventsumModifieddatetime;
    }

    public Long getInventtableRecid() {
        return inventtableRecid;
    }

    public String getBOMUnitId() {
        return bOMUnitId;
    }

    public BigDecimal getDensity() {
        return density;
    }

    public BigDecimal getDepth() {
        return depth;
    }

    public BigDecimal getGrossDepth() {
        return grossDepth;
    }

    public BigDecimal getGrossHeight() {
        return grossHeight;
    }

    public BigDecimal getGrossWidth() {
        return grossWidth;
    }

    public BigDecimal getHeight() {
        return height;
    }

    public String getNameAlias() {
        return nameAlias;
    }

    public BigDecimal getNetWeight() {
        return netWeight;
    }

    public String getPrimaryVendorId() {
        return primaryVendorId;
    }

    public Long getProduct() {
        return product;
    }

    public String getProjCategoryId() {
        return projCategoryId;
    }

    public String getSatcodeidMx() {
        return satcodeidMx;
    }

    public Instant getInventtableModifieddatetime() {
        return inventtableModifieddatetime;
    }

    public Long getEcoresproductRecid() {
        return ecoresproductRecid;
    }

    public String getDisplayProductNumber() {
        return displayProductNumber;
    }

    public String getSearchName() {
        return searchName;
    }

    public Long getProductmaster() {
        return productmaster;
    }

    public Long getEcoresproducttranslationRecid() {
        return ecoresproducttranslationRecid;
    }

    public String getDescription() {
        return description;
    }

    public String getName() {
        return name;
    }

    public Long getInventitemgroupitemRecid() {
        return inventitemgroupitemRecid;
    }

    public String getItemGroupId() {
        return itemGroupId;
    }

    protected EcommProducts() {
    }
}