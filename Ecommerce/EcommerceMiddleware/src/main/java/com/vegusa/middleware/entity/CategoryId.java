package com.vegusa.middleware.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Embeddable;
import org.hibernate.Hibernate;

import java.io.Serializable;
import java.util.Objects;

@Embeddable
public class CategoryId implements Serializable {
    private static final long serialVersionUID = -5430477375766415573L;
    @Column(name = "RecId", nullable = false)
    private Long recId;

    @Column(name = "Name", nullable = false, length = 50)
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
        CategoryId entity = (CategoryId) o;
        return Objects.equals(this.name, entity.name) &&
                Objects.equals(this.recId, entity.recId);
    }

    @Override
    public int hashCode() {
        return Objects.hash(name, recId);
    }

}