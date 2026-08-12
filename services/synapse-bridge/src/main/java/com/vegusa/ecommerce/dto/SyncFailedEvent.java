package com.vegusa.ecommerce.dto;

import java.time.Instant;

public record SyncFailedEvent(
        SourceSystem source,
        int offset,
        int pageSize,
        String error,
        Instant timestamp
) {
}
