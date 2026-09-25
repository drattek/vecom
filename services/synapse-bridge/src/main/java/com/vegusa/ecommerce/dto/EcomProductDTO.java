package com.vegusa.ecommerce.dto;

public record EcomProductDTO(
        SourceSystem source,
        Long id, // RECID (bigint en D365: los RecId superan el rango de int)
        String code, // Articulo
        String description, // Descripción
        String partNumber, // Num.Parte
        String group, // Grupo
        String brand // Marca
) {
}
