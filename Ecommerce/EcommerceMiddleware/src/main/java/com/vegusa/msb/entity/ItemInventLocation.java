package com.vegusa.msb.entity;

import jakarta.persistence.Column;
import jakarta.persistence.EmbeddedId;
import jakarta.persistence.Entity;
import org.hibernate.annotations.Immutable;
import org.hibernate.annotations.Nationalized;

import java.math.BigDecimal;

/**
 * Mapping for DB view
 */
@Entity
@Immutable
public class ItemInventLocation {
    @EmbeddedId
    private ItemInventLocationId id;

    @Nationalized
    @Column(name = "\"Descripción\"", length = 60)
    private String descripción;

    @Nationalized
    @Column(name = "\"Num.Parte\"", length = 20)
    private String numParte;

    @Nationalized
    @Column(name = "Sucursal", length = 10)
    private String sucursal;

    @Nationalized
    @Column(name = "Name", length = 60)
    private String name;

    @Nationalized
    @Column(name = "Grupo", length = 10)
    private String grupo;

    @Column(name = "Disponible", precision = 38, scale = 16)
    private BigDecimal disponible;

    @Column(name = "\"Costo Transaccion\"", precision = 32, scale = 16)
    private BigDecimal costoTransaccion;

    @Column(name = "\"Costo promedio\"", precision = 38, scale = 6)
    private BigDecimal costoPromedio;

    @Column(name = "\"Costo total\"", precision = 38, scale = 6)
    private BigDecimal costoTotal;

    @Nationalized
    @Column(name = "Dimension", length = 30)
    private String dimension;

    @Nationalized
    @Column(name = "\"Cateogría\"", length = 254)
    private String cateogría;

    @Nationalized
    @Column(name = "Marca", length = 1000)
    private String marca;

    @Nationalized
    @Column(name = "Address", length = 250)
    private String address;

    public ItemInventLocationId getId() {
        return id;
    }

    public void setId(ItemInventLocationId id) {
        this.id = id;
    }

    public String getDescripción() {
        return descripción;
    }

    public String getNumParte() {
        return numParte;
    }

    public String getSucursal() {
        return sucursal;
    }

    public String getName() {
        return name;
    }

    public String getGrupo() {
        return grupo;
    }

    public BigDecimal getDisponible() {
        return disponible;
    }

    public BigDecimal getCostoTransaccion() {
        return costoTransaccion;
    }

    public BigDecimal getCostoPromedio() {
        return costoPromedio;
    }

    public BigDecimal getCostoTotal() {
        return costoTotal;
    }

    public String getDimension() {
        return dimension;
    }

    public String getCateogría() {
        return cateogría;
    }

    public String getMarca() {
        return marca;
    }

    public String getAddress() {
        return address;
    }

    protected ItemInventLocation() {
    }
}