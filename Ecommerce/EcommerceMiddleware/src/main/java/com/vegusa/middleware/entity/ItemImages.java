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
@Table(name = "vw_veg_images_by_product")
public class ItemImages {
    @EmbeddedId
    private VwVegImagesByProductId id;

    @Column(name = "internal_code", nullable = false, length = 50)
    private String internalCode;

    @Column(name = "id_mv", nullable = false, length = 100)
    private String idMv;

    @Column(name = "image_url", nullable = false, length = 250)
    private String imageUrl;

    public VwVegImagesByProductId getId() {
        return id;
    }

    public void setId(VwVegImagesByProductId id) {
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

    protected ItemImages() {
    }
}