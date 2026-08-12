package com.vegusa.ecommerce.dto;

public record PageProcessedEvent(
        SourceSystem source,
        int page,
        int offset,
        int records
) {
}
