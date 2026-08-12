package com.vegusa.ecommerce.service;

import com.vegusa.ecommerce.dto.NissanExistenciasDTO;
import com.vegusa.ecommerce.dto.SourceSystem;
import com.vegusa.ecommerce.dto.SyncFailedEvent;
import com.vegusa.ecommerce.exception.SynapseConnectionException;
import com.vegusa.ecommerce.repository.NissanExistenciasRepository;
import lombok.RequiredArgsConstructor;
import org.springframework.retry.annotation.Backoff;
import org.springframework.retry.annotation.Recover;
import org.springframework.retry.annotation.Retryable;
import org.springframework.stereotype.Service;

import java.time.Instant;
import java.util.List;

@Service
@RequiredArgsConstructor
public class NissanExistenciasService {
    private final NissanExistenciasRepository repository;
    private final EventPublisher eventPublisher;

    @Retryable(
            retryFor = Exception.class,
            maxAttempts = 3,
            backoff = @Backoff(
                    delay = 10000,
                    multiplier = 2
            )
    )
    public List<NissanExistenciasDTO> loadPage(int offset, int pageSize){
        return repository.getPagedExistencias(offset, pageSize);
    }

    @Recover
    public void recover(Exception ex, int offset, int pageSize){
        eventPublisher.nissanExistenciasSyncFailed(
                new SyncFailedEvent(
                        SourceSystem.NISSAN,
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
