package com.vegusa.middleware.integrations.mercadolibre.service.stock;

import com.vegusa.middleware.integrations.mercadolibre.client.product.ProductMeliClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.product.ProductMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.entity.SyncItemMeli;
import com.vegusa.middleware.integrations.mercadolibre.repository.SyncItemMeliRepository;
import com.vegusa.middleware.service.StockService;
import com.vegusa.middleware.utils.CommonUtils;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.math.BigDecimal;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

@Service
public class StockMeliService {
    @Autowired
    private SyncItemMeliRepository syncItemMeliRepository;

    @Autowired
    private StockService stockService;

    @Autowired
    private CommonUtils commonUtils;

    @Autowired
    private ProductMeliClient productClient;

    public Mono<Void> updateStock(String dataAreaId){
        SyncItemMeli[] syncItems = syncItemMeliRepository.getItems(dataAreaId);
        List<String> itemIds = new ArrayList<>();
        for (SyncItemMeli syncItem : syncItems){
            itemIds.add(syncItem.getInternalCode());
        }

        Map<String, BigDecimal> stocks = stockService.getStocks(itemIds);

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            if (!syncItem.getStatus().equals("under_review")){
                try {
                    int product_stock = stocks.getOrDefault(syncItem.getInternalCode(), new BigDecimal("0.00")).intValue();
                    ProductMeliDTO product = new ProductMeliDTO();
                    product.setAvailableQuantity(product_stock);

                    if (product_stock != syncItem.getAvailable()){
                        return productClient.updateProduct(product, syncItem.getResponseId())
                                .doOnSuccess(result -> {
                                    System.out.println("Stock updated: " + syncItem.getInternalCode() + " with: " + product_stock);
                                    syncItem.setAvailable(product_stock);
                                    syncItem.setStatus(result.getStatus());

                                    syncItemMeliRepository.save(syncItem);
                                })
                                .doOnError(error -> {
                                    System.err.println("Error in stock item: " + syncItem.getInternalCode() + " - " + error.getMessage());
                                });
                    }

                    return Mono.empty();
                } catch (RuntimeException e){
                    System.err.println("Error: " + e.getMessage());
                }
            }
            return Mono.empty();
        }).then().doOnSuccess(e -> {
            System.out.println("Stock Mercado Libre updated successfully");
        });
    }
}
