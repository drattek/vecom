package com.vegusa.ecommerce.dto;

import java.math.BigDecimal;

public record ItemInventLocationDTO(
        SourceSystem source,
        String articulo,
        String descripcion,
        String numeroParte,
        String sucursal,
        String almacen,
        String nombreSucursal,
        String grupo,
        Integer disponible,
        BigDecimal costoTransaccion,
        BigDecimal costoPromedio,
        BigDecimal costoTotal,
        String dimension,
        String categoria,
        String marca,
        String direccion,
        String empresa
) {
}
