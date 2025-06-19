package com.vegusa.middleware.integrations.jumpseller.service;

import com.vegusa.middleware.entity.SyncWarehouse;
import com.vegusa.middleware.integrations.jumpseller.client.product.JumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.client.stock.JumpsellerStock;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerProductDto;
import com.vegusa.middleware.integrations.jumpseller.dto.Product;
import com.vegusa.middleware.integrations.jumpseller.dto.StockDto;
import com.vegusa.middleware.integrations.jumpseller.dto.StockProjection;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncProductJumpsellerRepository;
import com.vegusa.middleware.integrations.jumpseller.utils.ProductUtils;
import com.vegusa.middleware.repository.SyncWarehouseRepository;
import com.vegusa.msb.entity.ItemInventLocation;
import com.vegusa.msb.repository.ItemInventLocationRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.math.BigDecimal;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

@Service
public class JumpsellerStockService {
    private final JumpsellerStock jumpsellerStock;

    @Autowired
    private JumpsellerProduct jumpsellerProduct;

    @Autowired
    private SyncProductJumpsellerRepository syncProductJumpsellerRepository;

    @Autowired
    private ProductUtils productUtils;

    @Autowired
    private ItemInventLocationRepository itemInventLocationRepository;

    @Autowired
    private SyncWarehouseRepository syncWarehouseRepository;

    @Autowired
    public JumpsellerStockService(JumpsellerStock jumpsellerStock){
        this.jumpsellerStock = jumpsellerStock;
    }

    public Mono<List<StockDto>> getStock(){
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts("MSB");
        List<List<String>> chunks = productUtils.getChunks(syncItems, 50);

        return Flux.fromIterable(chunks).flatMap(jumpsellerStock::getStock)
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
        List<List<String>> batches = partitionList(itemIds, 1000);

        SyncWarehouse[] syncWarehouses = syncWarehouseRepository.getSyncWarehouse();
        List<String> warehouses = new ArrayList<>();
        for (SyncWarehouse warehouse: syncWarehouses){
            warehouses.add(warehouse.getName());
        }

        Map<String, BigDecimal> stocks = new HashMap<>();
        for (List<String> batch : batches) {
            System.out.println(String.join(",", warehouses));
            System.out.println(String.join(",", batch));
            List<StockProjection> results = itemInventLocationRepository.getStock(warehouses, batch);

            for (StockProjection result : results) {
                stocks.put(result.getArticulo(), result.getTotal());
            }
        }

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            try {
                int product_stock = stocks.getOrDefault(syncItem.getInternalCode(), new BigDecimal("0.00")).intValue();
                Product product = new Product();
                product.setName(syncItem.getName());
                product.setPrice(syncItem.getPrice());
                product.setStock(product_stock);
                product.setStatus("available");
                JumpsellerProductDto productDto = new JumpsellerProductDto(product);

                return jumpsellerProduct.updateProduct(syncItem.getResponseId(), productDto)
                        .doOnSuccess(result -> {
                            System.out.println("Stock updated: " + syncItem.getInternalCode() + " with: " + product_stock);
                            syncItem.setStock(product_stock);
                            syncProductJumpsellerRepository.save(syncItem);
                        })
                        .doOnError(error -> {
                            System.err.println("Error updating stock: " + syncItem.getInternalCode());
                        });
            } catch (RuntimeException e){
                System.err.println("Error item: " + syncItem.getInternalCode() + " - " + e.getMessage());
            }
            return Mono.empty();
        }).then().doOnSuccess(e -> {
            System.out.println("Stock updated successfully");
        });
    }

    public static <T> List<List<T>> partitionList(List<T> list, int batchSize) {
        List<List<T>> partitions = new ArrayList<>();
        for (int i = 0; i < list.size(); i += batchSize) {
            partitions.add(list.subList(i, Math.min(i + batchSize, list.size())));
        }
        return partitions;
    }
}
