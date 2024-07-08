package com.vegusa.veg_mv_integration_midd.veg_middleware.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Embeddable;
import org.hibernate.Hibernate;

import java.io.Serializable;
import java.util.Objects;

@Embeddable
public class VwVegImagesByProductId implements Serializable {
    private static final long serialVersionUID = -3452604487885388751L;
    @Column(name = "sync_products_id", nullable = false)
    private Long syncProductsId;

    @Column(name = "scraped_images_id", nullable = false)
    private Long scrapedImagesId;

    public Long getSyncProductsId() {
        return syncProductsId;
    }

    public Long getScrapedImagesId() {
        return scrapedImagesId;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || Hibernate.getClass(this) != Hibernate.getClass(o)) return false;
        VwVegImagesByProductId entity = (VwVegImagesByProductId) o;
        return Objects.equals(this.scrapedImagesId, entity.scrapedImagesId) &&
                Objects.equals(this.syncProductsId, entity.syncProductsId);
    }

    @Override
    public int hashCode() {
        return Objects.hash(scrapedImagesId, syncProductsId);
    }

}