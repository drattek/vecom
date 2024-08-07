package com.vegusa.veg_mv_integration_midd.veg_middleware.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Embeddable;
import org.hibernate.Hibernate;

import java.io.Serializable;
import java.util.Objects;

@Embeddable
public class CompanyId implements Serializable {
    private static final long serialVersionUID = -2642848502357993677L;
    @Column(name = "RecId", columnDefinition = "int UNSIGNED not null")
    private Long recId;

    @Column(name = "DataAreaId", nullable = false, length = 20)
    private String dataAreaId;

    public Long getRecId() {
        return recId;
    }

    public void setRecId(Long recId) {
        this.recId = recId;
    }

    public String getDataAreaId() {
        return dataAreaId;
    }

    public void setDataAreaId(String dataAreaId) {
        this.dataAreaId = dataAreaId;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || Hibernate.getClass(this) != Hibernate.getClass(o)) return false;
        CompanyId entity = (CompanyId) o;
        return Objects.equals(this.dataAreaId, entity.dataAreaId) &&
                Objects.equals(this.recId, entity.recId);
    }

    @Override
    public int hashCode() {
        return Objects.hash(dataAreaId, recId);
    }

}