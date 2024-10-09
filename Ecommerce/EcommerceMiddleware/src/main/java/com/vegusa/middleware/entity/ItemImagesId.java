package com.vegusa.middleware.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Embeddable;
import org.hibernate.Hibernate;

import java.io.Serializable;
import java.util.Objects;

@Embeddable
public class ItemImagesId implements Serializable {
    private static final long serialVersionUID = -3452604487885388751L;
    @Column(name = "SyncItemRecId", nullable = false)
    private Long syncItemRecId;

    @Column(name = "ScrapedImageRecId", nullable = false)
    private Long scrapedImageRecId;

    public Long getSyncItemRecId() {
        return syncItemRecId;
    }

    public Long getScrapedImageRecId() {
        return scrapedImageRecId;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || Hibernate.getClass(this) != Hibernate.getClass(o)) return false;
        ItemImagesId entity = (ItemImagesId) o;
        return Objects.equals(this.scrapedImageRecId, entity.scrapedImageRecId) &&
                Objects.equals(this.syncItemRecId, entity.syncItemRecId);
    }

    @Override
    public int hashCode() {
        return Objects.hash(scrapedImageRecId, syncItemRecId);
    }

}