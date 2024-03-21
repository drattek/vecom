package com.vegusa.veg_mv_integration_midd.msb.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import org.hibernate.annotations.Immutable;
import org.hibernate.annotations.Nationalized;

import java.math.BigDecimal;
import java.time.Instant;

/**
 * Mapping for DB view
 */
@Entity
@Immutable
public class InventSum {
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

    @Column(name = "LastUpdDateExpected")
    private Instant lastUpdDateExpected;

    @Column(name = "LastUpdDatePhysical")
    private Instant lastUpdDatePhysical;

    @Column(name = "OnOrder", precision = 32, scale = 16)
    private BigDecimal onOrder;

    @Column(name = "Ordered", precision = 32, scale = 16)
    private BigDecimal ordered;

    @Column(name = "PdsCWArrived", precision = 32, scale = 16)
    private BigDecimal pdsCWArrived;

    @Column(name = "PdsCWAvailOrdered", precision = 32, scale = 16)
    private BigDecimal pdsCWAvailOrdered;

    @Column(name = "PdsCWAvailPhysical", precision = 32, scale = 16)
    private BigDecimal pdsCWAvailPhysical;

    @Column(name = "PdsCWDeducted", precision = 32, scale = 16)
    private BigDecimal pdsCWDeducted;

    @Column(name = "PdsCWOnOrder", precision = 32, scale = 16)
    private BigDecimal pdsCWOnOrder;

    @Column(name = "PdsCWOrdered", precision = 32, scale = 16)
    private BigDecimal pdsCWOrdered;

    @Column(name = "PdsCWPhysicalInvent", precision = 32, scale = 16)
    private BigDecimal pdsCWPhysicalInvent;

    @Column(name = "PdsCWPicked", precision = 32, scale = 16)
    private BigDecimal pdsCWPicked;

    @Column(name = "PdsCWPostedQty", precision = 32, scale = 16)
    private BigDecimal pdsCWPostedQty;

    @Column(name = "PdsCWQuotationIssue", precision = 32, scale = 16)
    private BigDecimal pdsCWQuotationIssue;

    @Column(name = "PdsCWQuotationReceipt", precision = 32, scale = 16)
    private BigDecimal pdsCWQuotationReceipt;

    @Column(name = "PdsCWReceived", precision = 32, scale = 16)
    private BigDecimal pdsCWReceived;

    @Column(name = "PdsCWRegistered", precision = 32, scale = 16)
    private BigDecimal pdsCWRegistered;

    @Column(name = "PdsCWReservOrdered", precision = 32, scale = 16)
    private BigDecimal pdsCWReservOrdered;

    @Column(name = "PdsCWReservPhysical", precision = 32, scale = 16)
    private BigDecimal pdsCWReservPhysical;

    @Column(name = "PhysicalInvent", precision = 32, scale = 16)
    private BigDecimal physicalInvent;

    @Column(name = "PhysicalValue", precision = 32, scale = 16)
    private BigDecimal physicalValue;

    @Column(name = "PhysicalValueSecCur_RU", precision = 32, scale = 16)
    private BigDecimal physicalvalueseccurRu;

    @Column(name = "Picked", precision = 32, scale = 16)
    private BigDecimal picked;

    @Column(name = "PostedQty", precision = 32, scale = 16)
    private BigDecimal postedQty;

    @Column(name = "PostedValue", precision = 32, scale = 16)
    private BigDecimal postedValue;

    @Column(name = "PostedValueSecCur_RU", precision = 32, scale = 16)
    private BigDecimal postedvalueseccurRu;

    @Column(name = "QuotationIssue", precision = 32, scale = 16)
    private BigDecimal quotationIssue;

    @Column(name = "QuotationReceipt", precision = 32, scale = 16)
    private BigDecimal quotationReceipt;

    @Column(name = "Received", precision = 32, scale = 16)
    private BigDecimal received;

    @Column(name = "Registered", precision = 32, scale = 16)
    private BigDecimal registered;

    @Column(name = "ReservOrdered", precision = 32, scale = 16)
    private BigDecimal reservOrdered;

    @Column(name = "ReservPhysical", precision = 32, scale = 16)
    private BigDecimal reservPhysical;

    @Column(name = "IsExcludedFromInventoryValue")
    private Integer isExcludedFromInventoryValue;

    @Nationalized
    @Column(name = "configId", length = 50)
    private String configId;

    @Nationalized
    @Column(name = "InventBatchId", length = 30)
    private String inventBatchId;

    @Nationalized
    @Column(name = "InventColorId", length = 60)
    private String inventColorId;

    @Nationalized
    @Column(name = "InventGtdId_RU", length = 30)
    private String inventgtdidRu;

    @Nationalized
    @Column(name = "InventLocationId", length = 10)
    private String inventLocationId;

    @Nationalized
    @Column(name = "InventOwnerId_RU", length = 40)
    private String inventowneridRu;

    @Nationalized
    @Column(name = "InventProfileId_RU", length = 10)
    private String inventprofileidRu;

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
    @Column(name = "LicensePlateId", length = 25)
    private String licensePlateId;

    @Nationalized
    @Column(name = "wMSLocationId", length = 10)
    private String wMSLocationId;

    @Nationalized
    @Column(name = "WMSPALLETID", nullable = false, length = 1000)
    private String wmspalletid;

    @Nationalized
    @Column(name = "InventDimension1", length = 1)
    private String inventDimension1;

    @Nationalized
    @Column(name = "InventDimension2", length = 1)
    private String inventDimension2;

    @Nationalized
    @Column(name = "InventDimension3", length = 1)
    private String inventDimension3;

    @Nationalized
    @Column(name = "InventDimension4", length = 1)
    private String inventDimension4;

    @Nationalized
    @Column(name = "InventDimension5", length = 1)
    private String inventDimension5;

    @Nationalized
    @Column(name = "InventDimension6", length = 1)
    private String inventDimension6;

    @Nationalized
    @Column(name = "InventDimension7", length = 1)
    private String inventDimension7;

    @Nationalized
    @Column(name = "InventDimension8", length = 1)
    private String inventDimension8;

    @Column(name = "InventDimension9")
    private Instant inventDimension9;

    @Column(name = "INVENTDIMENSION9TZID", nullable = false)
    private Integer inventdimension9tzid;

    @Column(name = "InventDimension10", precision = 32, scale = 16)
    private BigDecimal inventDimension10;

    @Nationalized
    @Column(name = "InventDimension11", length = 1)
    private String inventDimension11;

    @Nationalized
    @Column(name = "InventDimension12", length = 1)
    private String inventDimension12;

    @Nationalized
    @Column(name = "DataAreaId", nullable = false, length = 4)
    private String dataAreaId;

    @Column(name = "PARTITION", nullable = false)
    private Long partition;

    @Column(name = "RECVERSION", nullable = false)
    private Integer recversion;

    @Column(name = "MODIFIEDDATETIME", nullable = false)
    private Instant modifieddatetime;

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

    public Instant getLastUpdDateExpected() {
        return lastUpdDateExpected;
    }

    public Instant getLastUpdDatePhysical() {
        return lastUpdDatePhysical;
    }

    public BigDecimal getOnOrder() {
        return onOrder;
    }

    public BigDecimal getOrdered() {
        return ordered;
    }

    public BigDecimal getPdsCWArrived() {
        return pdsCWArrived;
    }

    public BigDecimal getPdsCWAvailOrdered() {
        return pdsCWAvailOrdered;
    }

    public BigDecimal getPdsCWAvailPhysical() {
        return pdsCWAvailPhysical;
    }

    public BigDecimal getPdsCWDeducted() {
        return pdsCWDeducted;
    }

    public BigDecimal getPdsCWOnOrder() {
        return pdsCWOnOrder;
    }

    public BigDecimal getPdsCWOrdered() {
        return pdsCWOrdered;
    }

    public BigDecimal getPdsCWPhysicalInvent() {
        return pdsCWPhysicalInvent;
    }

    public BigDecimal getPdsCWPicked() {
        return pdsCWPicked;
    }

    public BigDecimal getPdsCWPostedQty() {
        return pdsCWPostedQty;
    }

    public BigDecimal getPdsCWQuotationIssue() {
        return pdsCWQuotationIssue;
    }

    public BigDecimal getPdsCWQuotationReceipt() {
        return pdsCWQuotationReceipt;
    }

    public BigDecimal getPdsCWReceived() {
        return pdsCWReceived;
    }

    public BigDecimal getPdsCWRegistered() {
        return pdsCWRegistered;
    }

    public BigDecimal getPdsCWReservOrdered() {
        return pdsCWReservOrdered;
    }

    public BigDecimal getPdsCWReservPhysical() {
        return pdsCWReservPhysical;
    }

    public BigDecimal getPhysicalInvent() {
        return physicalInvent;
    }

    public BigDecimal getPhysicalValue() {
        return physicalValue;
    }

    public BigDecimal getPhysicalvalueseccurRu() {
        return physicalvalueseccurRu;
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

    public BigDecimal getPostedvalueseccurRu() {
        return postedvalueseccurRu;
    }

    public BigDecimal getQuotationIssue() {
        return quotationIssue;
    }

    public BigDecimal getQuotationReceipt() {
        return quotationReceipt;
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

    public Integer getIsExcludedFromInventoryValue() {
        return isExcludedFromInventoryValue;
    }

    public String getConfigId() {
        return configId;
    }

    public String getInventBatchId() {
        return inventBatchId;
    }

    public String getInventColorId() {
        return inventColorId;
    }

    public String getInventgtdidRu() {
        return inventgtdidRu;
    }

    public String getInventLocationId() {
        return inventLocationId;
    }

    public String getInventowneridRu() {
        return inventowneridRu;
    }

    public String getInventprofileidRu() {
        return inventprofileidRu;
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

    public String getLicensePlateId() {
        return licensePlateId;
    }

    public String getWMSLocationId() {
        return wMSLocationId;
    }

    public String getWmspalletid() {
        return wmspalletid;
    }

    public String getInventDimension1() {
        return inventDimension1;
    }

    public String getInventDimension2() {
        return inventDimension2;
    }

    public String getInventDimension3() {
        return inventDimension3;
    }

    public String getInventDimension4() {
        return inventDimension4;
    }

    public String getInventDimension5() {
        return inventDimension5;
    }

    public String getInventDimension6() {
        return inventDimension6;
    }

    public String getInventDimension7() {
        return inventDimension7;
    }

    public String getInventDimension8() {
        return inventDimension8;
    }

    public Instant getInventDimension9() {
        return inventDimension9;
    }

    public Integer getInventdimension9tzid() {
        return inventdimension9tzid;
    }

    public BigDecimal getInventDimension10() {
        return inventDimension10;
    }

    public String getInventDimension11() {
        return inventDimension11;
    }

    public String getInventDimension12() {
        return inventDimension12;
    }

    public String getDataAreaId() {
        return dataAreaId;
    }

    public Long getPartition() {
        return partition;
    }

    public Integer getRecversion() {
        return recversion;
    }

    public Instant getModifieddatetime() {
        return modifieddatetime;
    }

    protected InventSum() {
    }
}