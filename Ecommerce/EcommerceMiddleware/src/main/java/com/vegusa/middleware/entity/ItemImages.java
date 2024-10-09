package com.vegusa.middleware.entity;

import jakarta.persistence.Column;
import jakarta.persistence.EmbeddedId;
import jakarta.persistence.Entity;
import jakarta.persistence.Table;
import org.hibernate.annotations.Immutable;

/**
 * Mapping for DB view
 */
@Entity
@Immutable
@Table(name = "ItemImages")
public class ItemImages {
    @EmbeddedId
    private ItemImagesId id;

    @Column(name = "ItemId", nullable = false, length = 50)
    private String itemId;

    @Column(name = "MvItemId", nullable = false, length = 100)
    private String mvItemId;

    @Column(name = "ImageUrl", nullable = false, length = 250)
    private String imageUrl;

    @Column(name = "PartNumberSearched", length = 100)
    private String partNumberSearched;

    @Column(name = "PartNumberFound", length = 100)
    private String partNumberFound;

    @Column(name = "InterfaceId", length = 20)
    private String interfaceId;

    @Column(name = "DataAreaId", nullable = false, length = 20)
    private String dataAreaId;

    //getters
    public ItemImagesId getId() {
        return id;
    }
    public String getItemId() {
        return itemId;
    }
    public String getMvItemId() {
        return mvItemId;
    }
    public String getImageUrl() {
        return imageUrl;
    }
    public String getPartNumberSearched(){ return partNumberSearched; }
    public String getPartNumberFound(){ return partNumberFound; }
    public String getInterfaceId(){ return interfaceId; }
    public String getDataAreaId() { return dataAreaId; }

    protected ItemImages() {
    }
}