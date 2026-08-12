package com.vegusa.ecommerce.service;

import com.vegusa.ecommerce.dto.*;
import lombok.RequiredArgsConstructor;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;

import java.util.List;

@Service
@RequiredArgsConstructor
public class ProductService {
    private final ItemStockService itemService;
    private final EcomProductService ecomProductService;
    private final NissanExistenciasService nissanExistenciasService;
    private final RedisProductService redisProductService;
    private final EventPublisher eventPublisher;

    //@Scheduled(cron = "${sync.items.cron-expression}")
    public void getItems(){
        int pageSize = 1000;
        int offset = 0;
        int totalRecord = 0;
        int totalPages = 0;

        while(true){
            List<EcomProductDTO> products = ecomProductService.loadPage(offset, pageSize);

            if (products.isEmpty()){
                break;
            }

            redisProductService.saveProducts(products);

            eventPublisher.itemPageProcessed(
                    new PageProcessedEvent(
                            SourceSystem.ERP,
                            offset / pageSize + 1,
                            offset,
                            products.size()
                    )
            );

            offset += pageSize;
            totalRecord += products.size();
            totalPages += 1;
        }

        eventPublisher.itemSyncCompleted(
                new SyncCompletedEvent(
                        SourceSystem.ERP,
                        totalRecord,
                        totalPages
                )
        );
    }

    //@Scheduled(cron = "${sync.stock.cron-expression}")
    public void getStockInfo(){
        int pageSize = 1000;
        int offset = 0;
        int totalRecord = 0;
        int totalPages = 0;

        while (true) {
            List<ItemInventLocationDTO> products = itemService.loadPage(offset, pageSize);

            if (products.isEmpty()){
                break;
            }

            redisProductService.saveStock(products);

            eventPublisher.stockPageProcessed(
                    new PageProcessedEvent(
                            SourceSystem.ERP,
                            offset / pageSize + 1,
                            offset,
                            products.size()
                    )
            );

            offset += pageSize;
            totalRecord += products.size();
            totalPages++;
        }

        eventPublisher.stockSyncCompleted(
                new SyncCompletedEvent(
                        SourceSystem.ERP,
                        totalRecord,
                        totalPages
                )
        );
    }

    //@Scheduled(cron = "${sync.existencias.cron-expression}")
    public void getExistenciasInfo(){
        int pageSize = 1000;
        int offset = 0;
        int totalRecord = 0;
        int totalPages = 0;

        while (true) {
            List<NissanExistenciasDTO> existencias = nissanExistenciasService.loadPage(offset, pageSize);

            if (existencias.isEmpty()){
                break;
            }

            redisProductService.saveExistencias(existencias);

            eventPublisher.nissanExistenciasPageProcessed(
                    new PageProcessedEvent(
                            SourceSystem.NISSAN,
                            offset / pageSize + 1,
                            offset,
                            existencias.size()
                    )
            );

            offset += pageSize;
            totalRecord += existencias.size();
            totalPages += 1;
        }

        eventPublisher.nissanExistenciasSyncCompleted(
                new SyncCompletedEvent(
                        SourceSystem.NISSAN,
                        totalRecord,
                        totalPages
                )
        );
    }
}
