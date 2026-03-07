package com.vegusa.middleware.integrations.jumpseller.service.stock;

import com.vegusa.middleware.integrations.jumpseller.client.product.ProductJumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.client.stock.StockJumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerProductDto;
import com.vegusa.middleware.integrations.jumpseller.dto.Product;
import com.vegusa.middleware.integrations.jumpseller.dto.StockDto;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncProductJumpsellerRepository;
import com.vegusa.middleware.integrations.jumpseller.utils.ProductUtils;
import com.vegusa.middleware.repository.erp.ItemInventLocationRepository;
import com.vegusa.middleware.repository.local.SyncWarehouseRepository;
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
public class StockJumpsellerService {
    private final StockJumpsellerClient stockJumpsellerClient;

    @Autowired
    private ProductJumpsellerClient productJumpsellerClient;

    @Autowired
    private SyncProductJumpsellerRepository syncProductJumpsellerRepository;

    @Autowired
    private ProductUtils productUtils;

    @Autowired
    private ItemInventLocationRepository itemInventLocationRepository;

    @Autowired
    private SyncWarehouseRepository syncWarehouseRepository;

    @Autowired
    private CommonUtils commonUtils;

    @Autowired
    private StockService stockService;

    @Autowired
    public StockJumpsellerService(StockJumpsellerClient stockJumpsellerClient){
        this.stockJumpsellerClient = stockJumpsellerClient;
    }

    public Mono<List<StockDto>> getStock(){
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts("MSB");
        List<List<String>> chunks = productUtils.getChunks(syncItems, 50);

        return Flux.fromIterable(chunks).flatMap(stockJumpsellerClient::getStock)
                .collectList()
                .map(listOfArrays -> {
                    List<StockDto> flatList = new ArrayList<>();
                    listOfArrays.forEach(array -> {
                        if (array != null) {
                            flatList.addAll(List.of(array));
                        }
                    });
                    return flatList;
                });
    }

    public Mono<Void> updateStock(String dataAreaId){
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts(dataAreaId);
        List<String> itemIds = new ArrayList<>();
        for (SyncJumpsellerProduct syncItem: syncItems){
            itemIds.add(syncItem.getInternalCode());
        }

        Map<String, BigDecimal> stocks = stockService.getStocks(itemIds);

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            try {
                int product_stock = stocks.getOrDefault(syncItem.getInternalCode(), new BigDecimal("0.00")).intValue();

                if (product_stock != syncItem.getStock()){
                    Product product = new Product();
                    product.setName(syncItem.getName());
                    product.setPrice(syncItem.getPrice());
                    product.setStock(product_stock);
                    product.setStatus("available");
                    JumpsellerProductDto productDto = new JumpsellerProductDto(product);

                    return productJumpsellerClient.updateProduct(syncItem.getResponseId(), productDto)
                            .doOnSuccess(result -> {
                                System.out.println("Stock updated: " + syncItem.getInternalCode() + " with: " + product_stock);
                                syncItem.setStock(product_stock);
                                syncProductJumpsellerRepository.save(syncItem);
                            })
                            .doOnError(error -> System.err.println("Error updating stock: " + syncItem.getInternalCode()));
                }
            } catch (RuntimeException e){
                System.err.println("Error item: " + syncItem.getInternalCode() + " - " + e.getMessage());
            }
            return Mono.empty();
        }).then().doOnSuccess(e -> System.out.println("Stock updated successfully"));
    }
}
