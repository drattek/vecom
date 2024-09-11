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

    @Column(name = "internal_code", nullable = false, length = 50)
    private String internalCode;

    @Column(name = "id_mv", nullable = false, length = 100)
    private String idMv;

    @Column(name = "image_url", nullable = false, length = 250)
    private String imageUrl;

    @Column(name = "veg_business_unit", nullable = false, length = 20)
    private String dataAreaId;

    public ItemImagesId getId() {
        return id;
    }

    public void setId(ItemImagesId id) {
        this.id = id;
    }

    public String getInternalCode() {
        return internalCode;
    }

    public String getIdMv() {
        return idMv;
    }

    public String getImageUrl() {
        return imageUrl;
    }

    public String getDataAreaId() { return dataAreaId; }

    protected ItemImages() {
    }
}