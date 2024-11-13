package com.vegusa.middleware.entity;

import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.persistence.*;

@Entity
@Table(name = "syncproductversion")
public class SyncProductVersion {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long id;

    @Column(name = "ResponseId", length = 100)
    @JsonProperty("_id")
    private String responseId;

    @Column(name = "ResponseStatus", length = 50)
    @JsonProperty("status")
    private String responseStatus;

    @Column(name = "Code", length = 50)
    @JsonProperty("code")
    private String code;

    @Column(name = "InternalCode", length = 50)
    @JsonProperty("internalCode")
    private String internalCode;

    @Column(name = "SizeId", length = 100)
    @JsonProperty("SizeId")
    private String sizeId;

    @Column(name = "ColorId", length = 100)
    @JsonProperty("ColorId")
    private String colorId;

    @Column(name = "Position", length = 10)
    @JsonProperty("position")
    private String position;

    @Column(name = "Width", length = 20)
    @JsonProperty("width")
    private String width;

    @Column(name = "Length", length = 20)
    @JsonProperty("length")
    private String length;

    @Column(name = "Height", length = 20)
    @JsonProperty("height")
    private String height;

    @Column(name = "Weight", length = 20)
    @JsonProperty("weight")
    private String weight;

    @Column(name = "CodeTypeId", length = 100)
    @JsonProperty("CodeTypeId")
    private String codeTypeId;

    @Column(name = "CodeType", length = 100)
    @JsonProperty("CodeType")
    private String codeType;

    @Column(name = "InternalCodeTypeId", length = 100)
    @JsonProperty("InternalCodeTypeId")
    private String internalCodeTypeId;

    @Column(name = "InternalCodeType", length = 100)
    @JsonProperty("InternalCodeType")
    private String internalCodeType;

    @Column(name = "InventoryTypeId", length = 100)
    @JsonProperty("InventoryTypeId")
    private String inventoryTypeId;

    @Column(name = "InventoryType", length = 100)
    @JsonProperty("InventoryType")
    private String inventoryType;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumns({
            @JoinColumn(name = "CompanyRefRecId", referencedColumnName = "RecId", nullable = false),
            @JoinColumn(name = "DataAreaId", referencedColumnName = "DataAreaId", nullable = false)
    })
    private Company company;

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getResponseId() {
        return responseId;
    }

    public void setResponseId(String responseId) {
        this.responseId = responseId;
    }

    public String getResponseStatus() {
        return responseStatus;
    }

    public void setResponseStatus(String responseStatus) {
        this.responseStatus = responseStatus;
    }

    public String getCode() {
        return code;
    }

    public void setCode(String code) {
        this.code = code;
    }

    public String getInternalCode() {
        return internalCode;
    }

    public void setInternalCode(String internalCode) {
        this.internalCode = internalCode;
    }

    public String getSizeId() {
        return sizeId;
    }

    public void setSizeId(String sizeId) {
        this.sizeId = sizeId;
    }

    public String getColorId() {
        return colorId;
    }

    public void setColorId(String colorId) {
        this.colorId = colorId;
    }

    public String getPosition() {
        return position;
    }

    public void setPosition(String position) {
        this.position = position;
    }

    public String getWidth() {
        return width;
    }

    public void setWidth(String width) {
        this.width = width;
    }

    public String getLength() {
        return length;
    }

    public void setLength(String length) {
        this.length = length;
    }

    public String getHeight() {
        return height;
    }

    public void setHeight(String height) {
        this.height = height;
    }

    public String getWeight() {
        return weight;
    }

    public void setWeight(String weight) {
        this.weight = weight;
    }

    public String getCodeTypeId() {
        return codeTypeId;
    }

    public void setCodeTypeId(String codeTypeId) {
        this.codeTypeId = codeTypeId;
    }

    public String getCodeType() {
        return codeType;
    }

    public void setCodeType(String codeType) {
        this.codeType = codeType;
    }

    public String getInternalCodeTypeId() {
        return internalCodeTypeId;
    }

    public void setInternalCodeTypeId(String internalCodeTypeId) {
        this.internalCodeTypeId = internalCodeTypeId;
    }

    public String getInternalCodeType() {
        return internalCodeType;
    }

    public void setInternalCodeType(String internalCodeType) {
        this.internalCodeType = internalCodeType;
    }

    public String getInventoryTypeId() {
        return inventoryTypeId;
    }

    public void setInventoryTypeId(String inventoryTypeId) {
        this.inventoryTypeId = inventoryTypeId;
    }

    public String getInventoryType() {
        return inventoryType;
    }

    public void setInventoryType(String inventoryType) {
        this.inventoryType = inventoryType;
    }

    public Company getCompany() {
        return company;
    }

    public void setCompany(Company company) {
        this.company = company;
    }

}