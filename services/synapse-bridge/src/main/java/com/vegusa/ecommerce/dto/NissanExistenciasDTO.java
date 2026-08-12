package com.vegusa.ecommerce.dto;

import java.math.BigDecimal;

public record NissanExistenciasDTO(
        SourceSystem source,
        String code, // PROD_CLAVE
        String type, //PROD_TIPOREFA
        Integer stock, // RELA_EXISTENCIAACTUAL
        BigDecimal costProm, // RELA_COSTOPROMEDIO
        String status, // PROD_STATUS
        String classification, // RELA_UBICACION
        String description, // PROD_DESCRIPCION1
        String unit, // PROD_UNIDAD
        String agencyName, // AGEN_NOMAGENCIA
        Boolean original, // PROD_ORIGINAL
        BigDecimal price, // PROD_PRECIO1
        BigDecimal price2, // PROD_PRECIO2
        BigDecimal price3, // PROD_PRECIO3
        BigDecimal price4, // PROD_PRECIO4
        BigDecimal price5, // PROD_PRECIO5
        String supersededByPartNumber // PROD_SUPERSESION: código de la pieza que reemplaza a esta
) {
}
