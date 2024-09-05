package com.vegusa.middleware.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Embeddable;
import org.hibernate.Hibernate;

import java.io.Serializable;
import java.util.Objects;

@Embeddable
public class ProductAttributeId implements Serializable {
    private static final long serialVersionUID = 3101705188987513779L;
    @Column(name = "RecId", columnDefinition = "int UNSIGNED not null")
    private Long recId;

    @Column(name = "ProductAttributeId", nullable = false, length = 50)
    private String productAttributeId;

    public Long getRecId() {
        return recId;
    }

    public void setRecId(Long recId) {
        this.recId = recId;
    }

    public String getProductAttributeId() {
        return productAttributeId;
    }

    public void setProductAttributeId(String productAttributeId) {
        this.productAttributeId = productAttributeId;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || Hibernate.getClass(this) != Hibernate.getClass(o)) return false;
        ProductAttributeId entity = (ProductAttributeId) o;
        return Objects.equals(this.productAttributeId, entity.productAttributeId) &&
                Objects.equals(this.recId, entity.recId);
    }

    @Override
    public int hashCode() {
        return Objects.hash(productAttributeId, recId);
    }

}