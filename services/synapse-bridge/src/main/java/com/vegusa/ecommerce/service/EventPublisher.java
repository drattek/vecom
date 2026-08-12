package com.vegusa.ecommerce.service;

import com.vegusa.ecommerce.config.RabbitConfig;
import com.vegusa.ecommerce.dto.PageProcessedEvent;
import com.vegusa.ecommerce.dto.SyncCompletedEvent;
import com.vegusa.ecommerce.dto.SyncFailedEvent;
import lombok.RequiredArgsConstructor;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class EventPublisher {
    private final RabbitTemplate rabbitTemplate;

    public void stockPageProcessed(PageProcessedEvent event){
        rabbitTemplate.convertAndSend(
                RabbitConfig.SYNC_EXCHANGE,
                "stock.page.processed",
                event
        );
    }

    public void stockSyncCompleted(SyncCompletedEvent event){
        rabbitTemplate.convertAndSend(
                RabbitConfig.SYNC_EXCHANGE,
                "stock.sync.completed",
                event
        );
    }

    public void stockSyncFailed(SyncFailedEvent event){
        rabbitTemplate.convertAndSend(
                RabbitConfig.SYNC_EXCHANGE,
                "stock.sync.failed",
                event
        );
    }

    public void itemPageProcessed(PageProcessedEvent event){
        rabbitTemplate.convertAndSend(
                RabbitConfig.SYNC_EXCHANGE,
                "item.page.processed",
                event
        );
    }

    public void itemSyncCompleted(SyncCompletedEvent event){
        rabbitTemplate.convertAndSend(
                RabbitConfig.SYNC_EXCHANGE,
                "item.sync.completed",
                event
        );
    }

    public void itemSyncFailed(SyncFailedEvent event){
        rabbitTemplate.convertAndSend(
                RabbitConfig.SYNC_EXCHANGE,
                "item.sync.failed",
                event
        );
    }

    public void nissanExistenciasPageProcessed(PageProcessedEvent event){
        rabbitTemplate.convertAndSend(
                RabbitConfig.SYNC_EXCHANGE,
                "nissan.existencias.page.processed",
                event
        );
    }

    public void nissanExistenciasSyncCompleted(SyncCompletedEvent event){
        rabbitTemplate.convertAndSend(
                RabbitConfig.SYNC_EXCHANGE,
                "nissan.existencias.sync.completed",
                event
        );
    }

    public void nissanExistenciasSyncFailed(SyncFailedEvent event){
        rabbitTemplate.convertAndSend(
                RabbitConfig.SYNC_EXCHANGE,
                "nissan.existencias.sync.failed",
                event
        );
    }
}
