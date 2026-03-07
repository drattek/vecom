package com.vegusa.middleware.model.erp;

import jakarta.persistence.Column;
import jakarta.persistence.Embeddable;
import org.hibernate.Hibernate;
import org.hibernate.annotations.Nationalized;

import java.io.Serializable;
import java.util.Objects;

@Embeddable
public class ItemInventLocationId implements Serializable {
    private static final long serialVersionUID = 3124139142363801246L;
    @Nationalized
    @Column(name = "Articulo", nullable = false, length = 20)
    private String articulo;

    @Nationalized
    @Column(name = "Almacen", length = 10)
    private String almacen;

    public String getArticulo() {
        return articulo;
    }

    public String getAlmacen() {
        return almacen;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || Hibernate.getClass(this) != Hibernate.getClass(o)) return false;
        ItemInventLocationId entity = (ItemInventLocationId) o;
        return Objects.equals(this.almacen, entity.almacen) &&
                Objects.equals(this.articulo, entity.articulo);
    }

    @Override
    public int hashCode() {
        return Objects.hash(almacen, articulo);
    }

}