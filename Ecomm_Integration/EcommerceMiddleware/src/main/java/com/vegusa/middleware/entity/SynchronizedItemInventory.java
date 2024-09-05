package com.vegusa.middleware.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "synchronizediteminventory")
public class SynchronizedItemInventory {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long id;

    @Column(name = "IdEcom", length = 100)
    private String idEcom;

    @Column(name = "Code", length = 100)
    private String code;

    @Column(name = "WarehouseId", length = 100)
    private String warehouseId;

    @Column(name = "ItemRelocationId", length = 100)
    private String itemRelocationId;

    @Column(name = "ItemRelocationAmount", length = 50)
    private String itemRelocationAmount;

    @Column(name = "ItemRelocationType", length = 50)
    private String itemRelocationType;

    @Column(name = "ItemRelocationCategoryId", length = 100)
    private String itemRelocationCategoryId;

    @Column(name = "AvailableProdStockId", length = 100)
    private String availableProdStockId;

    @Column(name = "AvailableProdStockAmount", length = 50)
    private String availableProdStockAmount;

    @Column(name = "Success", length = 10)
    private String success;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    private Company company;

    @Column(name = "ErrorItemAmount", length = 50)
    private String errorItemAmount;

    @Column(name = "ErrorMessage", length = 100)
    private String errorMessage;

    public String getErrorMessage() {
        return errorMessage;
    }

    public void setErrorMessage(String errorMessage) {
        this.errorMessage = errorMessage;
    }

    public String getErrorItemAmount() {
        return errorItemAmount;
    }

    public void setErrorItemAmount(String errorItemAmount) {
        this.errorItemAmount = errorItemAmount;
    }

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getIdEcom() {
        return idEcom;
    }

    public void setIdEcom(String idEcom) {
        this.idEcom = idEcom;
    }

    public String getCode() {
        return code;
    }

    public void setCode(String code) {
        this.code = code;
    }

    public String getWarehouseId() {
        return warehouseId;
    }

    public void setWarehouseId(String warehouseId) {
        this.warehouseId = warehouseId;
    }

    public String getItemRelocationId() {
        return itemRelocationId;
    }

    public void setItemRelocationId(String itemRelocationId) {
        this.itemRelocationId = itemRelocationId;
    }

    public String getItemRelocationAmount() {
        return itemRelocationAmount;
    }

    public void setItemRelocationAmount(String itemRelocationAmount) {
        this.itemRelocationAmount = itemRelocationAmount;
    }

    public String getItemRelocationType() {
        return itemRelocationType;
    }

    public void setItemRelocationType(String itemRelocationType) {
        this.itemRelocationType = itemRelocationType;
    }

    public String getItemRelocationCategoryId() {
        return itemRelocationCategoryId;
    }

    public void setItemRelocationCategoryId(String itemRelocationCategoryId) {
        this.itemRelocationCategoryId = itemRelocationCategoryId;
    }

    public String getAvailableProdStockId() {
        return availableProdStockId;
    }

    public void setAvailableProdStockId(String availableProdStockId) {
        this.availableProdStockId = availableProdStockId;
    }

    public String getAvailableProdStockAmount() {
        return availableProdStockAmount;
    }

    public void setAvailableProdStockAmount(String availableProdStockAmount) {
        this.availableProdStockAmount = availableProdStockAmount;
    }

    public String getSuccess() {
        return success;
    }

    public void setSuccess(String success) {
        this.success = success;
    }

    public Company getCompany() {
        return company;
    }

    public void setCompany(Company company) {
        this.company = company;
    }

}