package com.vegusa.middleware.entity;

import jakarta.persistence.EmbeddedId;
import jakarta.persistence.Entity;
import jakarta.persistence.Table;

@Entity
@Table(name = "profitmargincategory")
public class ProfitMarginCategory {
    @EmbeddedId
    private ProfitMarginCategoryId id;

    public ProfitMarginCategoryId getId() {
        return id;
    }

    public void setId(ProfitMarginCategoryId id) {
        this.id = id;
    }

    //TODO [Reverse Engineering] generate columns from DB
}