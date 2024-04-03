package com.vegusa.veg_mv_integration_midd.msb.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Embeddable;
import org.hibernate.Hibernate;

import java.io.Serializable;
import java.util.Objects;

@Embeddable
public class EcommProductId implements Serializable {
    private static final long serialVersionUID = 7221823069185418843L;
    @Column(name = "InventSum_RECID", nullable = false)
    private Long inventsumRecid;

    @Column(name = "InventTable_RECID", nullable = false)
    private Long inventtableRecid;

    @Column(name = "EcoResProduct_RECID", nullable = false)
    private Long ecoresproductRecid;

    @Column(name = "EcoResProductTranslation_RECID")
    private Long ecoresproducttranslationRecid;

    public Long getInventsumRecid() {
        return inventsumRecid;
    }

    public Long getInventtableRecid() {
        return inventtableRecid;
    }

    public Long getEcoresproductRecid() {
        return ecoresproductRecid;
    }

    public Long getEcoresproducttranslationRecid() {
        return ecoresproducttranslationRecid;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || Hibernate.getClass(this) != Hibernate.getClass(o)) return false;
        EcommProductId entity = (EcommProductId) o;
        return Objects.equals(this.ecoresproductRecid, entity.ecoresproductRecid) &&
                Objects.equals(this.inventtableRecid, entity.inventtableRecid) &&
                Objects.equals(this.ecoresproducttranslationRecid, entity.ecoresproducttranslationRecid) &&
                Objects.equals(this.inventsumRecid, entity.inventsumRecid);
    }

    @Override
    public int hashCode() {
        return Objects.hash(ecoresproductRecid, inventtableRecid, ecoresproducttranslationRecid, inventsumRecid);
    }

}