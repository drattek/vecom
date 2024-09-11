package com.vegusa.middleware.entity;

import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.persistence.*;

import java.util.Date;

@Entity
@Table(name = "SyncWarehouse")
public class SyncWarehouse {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long id;

    @JsonProperty("_id")
    @Column(name = "IdEcom", length = 100)
    private String idEcom;

    @JsonProperty("status")
    @Column(name = "Status", length = 50)
    private String status;

    @JsonProperty("name")
    @Column(name = "Name", length = 50)
    private String name;

    @JsonProperty("description")
    @Column(name = "Description", length = 100)
    private String description;

    @JsonProperty("address")
    @Column(name = "Address", length = 250)
    private String address;

    @JsonProperty("type")
    @Column(name = "Type", length = 50)
    private String type;

    @JsonProperty("phoneAreaCode")
    @Column(name = "PhoneAreaCode", length = 20)
    private String phoneAreaCode;

    @JsonProperty("phoneNumber")
    @Column(name = "PhoneNumber", length = 20)
    private String phoneNumber;

    @JsonProperty("latitude")
    @Column(name = "Latitude", length = 20)
    private String latitude;

    @JsonProperty("longitude")
    @Column(name = "Longitude", length = 20)
    private String longitude;

    @JsonProperty("openHours")
    @Column(name = "OpenHours", length = 20)
    private String openHours;

    @JsonProperty("CreatedById")
    @Column(name = "CreatedById", length = 100)
    private String createdById;

    @JsonProperty("UpdatedById")
    @Column(name = "UpdatedById", length = 100)
    private String updatedById;

    @JsonProperty("MerchantId")
    @Column(name = "MerchantId", length = 100)
    private String merchantId;

    @JsonProperty("position")
    @Column(name = "Position", length = 20)
    private String position;

    @JsonProperty("createdAt")
    @Column(name = "created_at")
    private Date createdAt;

    @JsonProperty("updatedAt")
    @Column(name = "updated_at")
    private Date updatedAt;

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

    public String getIdEcom() {
        return idEcom;
    }

    public void setIdEcom(String idEcom) {
        this.idEcom = idEcom;
    }

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getAddress() {
        return address;
    }

    public void setAddress(String address) {
        this.address = address;
    }

    public String getType() {
        return type;
    }

    public void setType(String type) {
        this.type = type;
    }

    public String getPhoneAreaCode() {
        return phoneAreaCode;
    }

    public void setPhoneAreaCode(String phoneAreaCode) {
        this.phoneAreaCode = phoneAreaCode;
    }

    public String getPhoneNumber() {
        return phoneNumber;
    }

    public void setPhoneNumber(String phoneNumber) {
        this.phoneNumber = phoneNumber;
    }

    public String getLatitude() {
        return latitude;
    }

    public void setLatitude(String latitude) {
        this.latitude = latitude;
    }

    public String getLongitude() {
        return longitude;
    }

    public void setLongitude(String longitude) {
        this.longitude = longitude;
    }

    public String getOpenHours() {
        return openHours;
    }

    public void setOpenHours(String openHours) {
        this.openHours = openHours;
    }

    public String getCreatedById() {
        return createdById;
    }

    public void setCreatedById(String createdById) {
        this.createdById = createdById;
    }

    public String getUpdatedById() {
        return updatedById;
    }

    public void setUpdatedById(String updatedById) {
        this.updatedById = updatedById;
    }

    public String getMerchantId() {
        return merchantId;
    }

    public void setMerchantId(String merchantId) {
        this.merchantId = merchantId;
    }

    public String getPosition() {
        return position;
    }

    public void setPosition(String position) {
        this.position = position;
    }

    public Date getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(Date createdAt) {
        this.createdAt = createdAt;
    }

    public Date getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(Date updatedAt) {
        this.updatedAt = updatedAt;
    }

    public Company getCompany() {
        return company;
    }

    public void setCompany(Company company) {
        this.company = company;
    }

}