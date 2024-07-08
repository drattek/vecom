package com.vegusa.veg_mv_integration_midd.msb.entity;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import org.hibernate.annotations.Immutable;
import org.hibernate.annotations.Nationalized;

import java.math.BigDecimal;

/**
 * Mapping for DB view
 */
@Entity
@Immutable
@Table(name = "ECOMProducts")
public class ECOMProduct {
    @Id
    @Nationalized
    @Column(name = "Articulo", nullable = false, length = 20)
    private String articulo;

    @Nationalized
    @Column(name = "\"Descripción\"", length = 60)
    private String descripción;

    @Nationalized
    @Column(name = "\"Num.Parte\"", length = 20)
    private String numParte;

    @Nationalized
    @Column(name = "Grupo", length = 10)
    private String grupo;

    @Column(name = "Disponible", precision = 38, scale = 16)
    private BigDecimal disponible;

    @Column(name = "Costo", precision = 38, scale = 16)
    private BigDecimal costo;

    @Nationalized
    @Column(name = "Dimension", length = 30)
    private String dimension;

    @Nationalized
    @Column(name = "\"Cateogría\"", length = 254)
    private String cateogría;

    @Nationalized
    @Column(name = "Marca", length = 1000)
    private String marca;

    public String getArticulo() {
        return articulo;
    }

    public String getDescripción() {
        return descripción;
    }

    public String getNumParte() {
        return numParte;
    }

    public String getGrupo() {
        return grupo;
    }

    public BigDecimal getDisponible() {
        return disponible;
    }

    public BigDecimal getCosto() {
        return costo;
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

    protected ECOMProduct() {
    }
}