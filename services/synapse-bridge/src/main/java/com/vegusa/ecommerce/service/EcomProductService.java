package com.vegusa.ecommerce.service;

import com.vegusa.ecommerce.dto.EcomProductDTO;
import com.vegusa.ecommerce.dto.SourceSystem;
import com.vegusa.ecommerce.dto.SyncFailedEvent;
import com.vegusa.ecommerce.exception.SynapseConnectionException;
import com.vegusa.ecommerce.repository.EcomProductRepository;
import lombok.RequiredArgsConstructor;
import org.springframework.retry.annotation.Backoff;
import org.springframework.retry.annotation.Recover;
import org.springframework.retry.annotation.Retryable;
import org.springframework.stereotype.Service;

import java.time.Instant;
import java.util.List;

@Service
@RequiredArgsConstructor
public class EcomProductService {
    private final EcomProductRepository repository;
    private final EventPublisher eventPublisher;

    @Retryable(
            retryFor = Exception.class,
            maxAttempts = 3,
            backoff = @Backoff(
                    delay = 10000,
                    multiplier = 2
            )
    )
    public List<EcomProductDTO> loadPage(int offset, int pageSize){
        return repository.getPagedProducts(offset, pageSize);
    }

    @Recover
    public void recover(Exception ex, int offset, int pageSize){
        eventPublisher.itemSyncFailed(
                new SyncFailedEvent(
                        SourceSystem.ERP,
                        offset,
                        pageSize,
                        ex.getMessage(),
                        Instant.now()
                )
        );

        throw new SynapseConnectionException(
                offset,
                ex
        );
    }
}
