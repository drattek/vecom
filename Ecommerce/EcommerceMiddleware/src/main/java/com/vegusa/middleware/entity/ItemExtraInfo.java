package com.vegusa.middleware.entity;

import jakarta.persistence.*;
import org.hibernate.annotations.Immutable;

import java.time.Instant;

/**
 * Mapping for DB view
 */
@Entity
@Immutable
@Table(name = "itemextrainfo")
public class ItemExtraInfo {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long recId;

    @Column(name = "ItemId", nullable = false, length = 100)
    private String itemId;

    @Column(name = "ItemName", length = 250)
    private String itemName;

    @Column(name = "PartNumberSearched", length = 100)
    private String partNumberSearched;

    @Column(name = "PartNumberFound", length = 100)
    private String partNumberFound;

    @Column(name = "ShortDescription", length = 250)
    private String shortDescription;

    @Column(name = "Weight", length = 20)
    private String weight;

    @Column(name = "UnitOfMeasure", length = 20)
    private String unitOfMeasure;

    @Lob
    @Column(name = "CrossReferences")
    private String crossReferences;

    @Column(name = "ValidationReason", length = 20)
    private String validationReason;

    @Column(name = "ContractPrice", length = 20)
    private String contractPrice;

    @Column(name = "ImageUrl", length = 100)
    private String imageUrl;

    @Column(name = "OemId", length = 20)
    private String oemId;

    @Column(name = "FleetListPrice", length = 20)
    private String fleetListPrice;

    @Column(name = "DealerNetPrice", length = 20)
    private String dealerNetPrice;

    @Column(name = "Company", length = 20)
    private String company;

    @Column(name = "MktDescription", length = 250)
    private String mktDescription;

    @Column(name = "Category", length = 20)
    private String category;

    @Column(name = "InterfaceId", length = 20)
    private String interfaceId;

    @Column(name = "UpdatedAt")
    private Instant updatedAt;

    @Column(name = "CreatedAt")
    private Instant createdAt;

    @Column(name = "DataAreaId", nullable = false, length = 50)
    private String dataAreaId;

    public String getItemId() {
        return itemId;
    }

    public String getItemName() {
        return itemName;
    }

    public String getPartNumberSearched() {
        return partNumberSearched;
    }

    public String getPartNumberFound() {
        return partNumberFound;
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

    public String getCrossReferences() {
        return crossReferences;
    }

    public String getValidationReason() {
        return validationReason;
    }

    public String getContractPrice() {
        return contractPrice;
    }

    public String getImageUrl() {
        return imageUrl;
    }

    public String getOemId() {
        return oemId;
    }

    public String getFleetListPrice() {
        return fleetListPrice;
    }

    public String getDealerNetPrice() {
        return dealerNetPrice;
    }

    public String getCompany() {
        return company;
    }

    public String getMktDescription() {
        return mktDescription;
    }

    public String getCategory() {
        return category;
    }

    public String getInterfaceId() {
        return interfaceId;
    }

    public Instant getUpdatedAt() {
        return updatedAt;
    }

    public Instant getCreatedAt() {
        return createdAt;
    }

    public String getDataAreaId() {
        return dataAreaId;
    }

    protected ItemExtraInfo() {
    }
}