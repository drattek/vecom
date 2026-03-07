package com.vegusa.middleware.model.erp;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import org.hibernate.annotations.Immutable;

@Entity
@Immutable
@Table(name = "ECOMProducts", schema = "dyn")
public class EcomProduct {
    @Id
    @Column(name = "RECID", nullable = false)
    private long id;

    @Column(name = "Articulo", nullable = false, length = 20)
    private String itemId;

    public EcomProduct() {}

    public long getId() {
        return id;
    }

    public String getItemId() {
        return itemId;
    }
}
