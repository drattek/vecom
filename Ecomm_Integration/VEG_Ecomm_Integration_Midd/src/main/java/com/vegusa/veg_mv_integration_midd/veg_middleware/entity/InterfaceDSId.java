package com.vegusa.veg_mv_integration_midd.veg_middleware.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Embeddable;
import org.hibernate.Hibernate;

import java.io.Serializable;
import java.util.Objects;

@Embeddable
public class InterfaceDSId implements Serializable {
    private static final long serialVersionUID = 3558668203773103871L;
    @Column(name = "RecId", columnDefinition = "int UNSIGNED not null")
    private Long recId;

    @Column(name = "InterfaceId", nullable = false, length = 20)
    private String interfaceId;

    public Long getRecId() {
        return recId;
    }

    public void setRecId(Long recId) {
        this.recId = recId;
    }

    public String getInterfaceId() {
        return interfaceId;
    }

    public void setInterfaceId(String interfaceId) {
        this.interfaceId = interfaceId;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || Hibernate.getClass(this) != Hibernate.getClass(o)) return false;
        InterfaceDSId entity = (InterfaceDSId) o;
        return Objects.equals(this.interfaceId, entity.interfaceId) &&
                Objects.equals(this.recId, entity.recId);
    }

    @Override
    public int hashCode() {
        return Objects.hash(interfaceId, recId);
    }

}