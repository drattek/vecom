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
public class InventItemGroupItem {
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
    @Column(name = "ItemDataAreaId", nullable = false, length = 4)
    private String itemDataAreaId;

    @Nationalized
    @Column(name = "ItemGroupDataAreaId", length = 4)
    private String itemGroupDataAreaId;

    @Nationalized
    @Column(name = "ItemGroupId", length = 10)
    private String itemGroupId;

    @Nationalized
    @Column(name = "ItemId", nullable = false, length = 20)
    private String itemId;

    @Column(name = "PARTITION", nullable = false)
    private Long partition;

    @Column(name = "RECVERSION", nullable = false)
    private Integer recversion;

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

    public String getItemDataAreaId() {
        return itemDataAreaId;
    }

    public String getItemGroupDataAreaId() {
        return itemGroupDataAreaId;
    }

    public String getItemGroupId() {
        return itemGroupId;
    }

    public String getItemId() {
        return itemId;
    }

    public Long getPartition() {
        return partition;
    }

    public Integer getRecversion() {
        return recversion;
    }

    protected InventItemGroupItem() {
    }
}