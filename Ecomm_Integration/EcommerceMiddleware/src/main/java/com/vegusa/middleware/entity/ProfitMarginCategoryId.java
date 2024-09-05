package com.vegusa.middleware.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Embeddable;
import org.hibernate.Hibernate;

import java.io.Serializable;
import java.util.Objects;

@Embeddable
public class ProfitMarginCategoryId implements Serializable {
    private static final long serialVersionUID = -9077186485360608462L;
    @Column(name = "RecId", columnDefinition = "int UNSIGNED not null")
    private Long recId;

    @Column(name = "Name", nullable = false, length = 20)
    private String name;

    public Long getRecId() {
        return recId;
    }

    public void setRecId(Long recId) {
        this.recId = recId;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || Hibernate.getClass(this) != Hibernate.getClass(o)) return false;
        ProfitMarginCategoryId entity = (ProfitMarginCategoryId) o;
        return Objects.equals(this.name, entity.name) &&
                Objects.equals(this.recId, entity.recId);
    }

    @Override
    public int hashCode() {
        return Objects.hash(name, recId);
    }

}