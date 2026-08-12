package com.vegusa.ecommerce.dto;

public record SyncCompletedEvent(
        SourceSystem source,
        int totalRecord,
        int pages
) {
}
