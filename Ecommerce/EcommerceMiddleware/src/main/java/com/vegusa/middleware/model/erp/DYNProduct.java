package com.vegusa.middleware.model.erp;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import org.hibernate.annotations.Immutable;
import org.hibernate.annotations.Nationalized;

import java.math.BigDecimal;
import java.util.Date;

/**
 * Mapping for DB view
 */
@Entity
@Immutable
@Table(name = "ECOMProducts")
public class DYNProduct {
    @Column(name = "Articulo", nullable = false, length = 20)
    private String articulo;

    @Column(name = "\"Descripción\"", length = 60)
    private String descripcion;

    @Column(name = "\"Num.Parte\"", length = 20)
    private String numParte;

    @Column(name = "Grupo", length = 10)
    private String grupo;

    @Column(name = "Disponible", precision = 38, scale = 16)
    private BigDecimal disponible;

    @Column(name = "Costo", precision = 38, scale = 16)
    private BigDecimal costo;

    @Column(name = "Dimension", length = 30)
    private String dimension;

    @Column(name = "\"Cateogría\"", length = 254)
    private String categoria;

    @Column(name = "Marca", length = 1000)
    private String marca;

    @Column(name = "MODIFIEDDATETIME", nullable = false)
    private Date modifieddatetime;

    @Column(name = "NameAlias", length = 20)
    private String nameAlias;

    @Id
    @Column(name = "RECID", nullable = false)
    private Long recid;

    public Long getRecid() {
        return recid;
    }

    public String getNameAlias() {
        return nameAlias;
    }

    public Date getModifieddatetime() {
        return modifieddatetime;
    }

    public String getArticulo() {
        return articulo;
    }

    public String getDescripcion() {
        return descripcion;
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

    public String getCateogria() {
        return categoria;
    }

    public String getMarca() {
        return marca;
    }

    protected DYNProduct() {
    }
}