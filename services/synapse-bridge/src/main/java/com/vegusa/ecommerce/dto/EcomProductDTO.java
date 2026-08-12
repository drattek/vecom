package com.vegusa.ecommerce.dto;

public record EcomProductDTO(
        SourceSystem source,
        Integer id, // RECID
        String code, // Articulo
        String description, // Descripción
        String partNumber, // Num.Parte
        String group, // Grupo
        String brand // Marca
) {
}
