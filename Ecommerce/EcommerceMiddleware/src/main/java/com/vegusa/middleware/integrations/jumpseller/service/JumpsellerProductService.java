package com.vegusa.middleware.integrations.jumpseller.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.vegusa.middleware.entity.SyncItem;
import com.vegusa.middleware.integrations.jumpseller.client.product.JumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerProductDto;
import com.vegusa.middleware.integrations.jumpseller.dto.Product;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncProductJumpsellerRepository;
import com.vegusa.middleware.integrations.jumpseller.utils.ProductUtils;
import com.vegusa.middleware.repository.SyncItemRepository;
import org.json.JSONArray;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.List;
import java.util.stream.IntStream;

@Service
public class JumpsellerProductService {

    private final JumpsellerProduct jumpsellerClient;

    @Autowired
    private ProductUtils productUtils;

    @Autowired
    private SyncProductJumpsellerRepository syncProductJumpsellerRepository;

    @Autowired
    private SyncItemRepository syncItemRepository;

    @Autowired
    public JumpsellerProductService(JumpsellerProduct jumpsellerClient) {
        this.jumpsellerClient = jumpsellerClient;
    }

    public Mono<JumpsellerProductDto> createProduct(JumpsellerProductDto productDto) {
        return jumpsellerClient.createProduct(productDto);
    }

    public Mono<JumpsellerProductDto> updateProduct(long id) {
        SyncJumpsellerProduct syncItem = syncProductJumpsellerRepository.getSyncById(id);
        Product product = new Product();
        product.setName(syncItem.getName());
        product.setPrice(syncItem.getPrice());
        JumpsellerProductDto productDto = new JumpsellerProductDto(product);
        return jumpsellerClient.updateProduct(id, productDto);
    }

    public Mono<Void> deleteProduct(String id) {
        return jumpsellerClient.deleteProduct(id);
    }

    public Mono<JumpsellerProductDto> getProduct(long id) {
        return jumpsellerClient.getProductById(id);
    }

    public Mono<List<JumpsellerProductDto>> getAllProducts() {
        return jumpsellerClient.getProductCount().flatMapMany(count -> {
                    int totalPages = (int) Math.ceil(count / 100.0);
                    List<Integer> pages = IntStream.rangeClosed(1, totalPages).boxed().toList();

                    return Flux.fromIterable(pages)
                            .flatMap(jumpsellerClient::getAllProducts);
                }).collectList()
                .map(listOfArrays -> {
                    List<JumpsellerProductDto> flatList = new ArrayList<>();
                    listOfArrays.forEach(array -> {
                        if (array != null) {
                            flatList.addAll(List.of(array));
                        }
                    });
                    return flatList;
                })
                .doOnNext(products -> {
                    for (JumpsellerProductDto product : products){
                        SyncItem syncItem = syncItemRepository.getSyncItemByName(product.getProduct().getName(), product.getProduct().getSku(), "MSB");
                        if (syncItem == null){
                            System.err.println(product.getProduct().getSku());
                            SyncJumpsellerProduct productEntity = productUtils.toEntity(product, product.getProduct().getSku());
                            syncProductJumpsellerRepository.save(productEntity);
                        } else {
                            SyncJumpsellerProduct productEntity = productUtils.toEntity(product, syncItem.getInternalCode());
                            syncProductJumpsellerRepository.save(productEntity);
                        }
                    }
                })
                .doOnSuccess(products -> {
                    System.out.println("Actualización concluida");
                });
    }

    public Mono<Void> updatePrices(JSONArray pricelist, String dataAreaId){
        HashMap<String, String> itemCostMap = productUtils.getPrices(pricelist);
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts(dataAreaId);

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            String costItem = itemCostMap.get(syncItem.getInternalCode());
            if (costItem != null){
                Product product = new Product();
                product.setName(syncItem.getName()); // Is required
                product.setPrice(Double.valueOf(costItem));
                JumpsellerProductDto productDto = new JumpsellerProductDto(product);

                return jumpsellerClient.updateProduct(syncItem.getResponseId(), productDto)
                        .doOnSuccess(result -> {
                            System.out.println("Updated product " + syncItem.getInternalCode() + " with price " + costItem);
                        })
                        .doOnError(error -> {
                            System.err.println("Error updating product " + syncItem.getInternalCode() + " : " + error.getMessage());
                        });
            }

            return Mono.empty();
        }).then().doOnSuccess(e -> {
            System.out.println("Pricelist Jumpseller updated ended");
        });
    }
}

