package com.vegusa.middleware.integrations.jumpseller.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.vegusa.middleware.entity.ProductAttributeValue;
import com.vegusa.middleware.entity.SyncItem;
import com.vegusa.middleware.integrations.jumpseller.client.product.JumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerProductDto;
import com.vegusa.middleware.integrations.jumpseller.dto.Product;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncProductJumpsellerRepository;
import com.vegusa.middleware.integrations.jumpseller.utils.ProductUtils;
import com.vegusa.middleware.repository.*;
import org.json.JSONArray;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.List;
import java.util.stream.Collectors;
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
    private ProductAttributeRepository productAttributeRepository;

    @Autowired
    private ProductAttributeHierarchyRepository productAttributeHierarchyRepository;

    @Autowired
    private InterfaceHierarchyRepository interfaceHierarchyRepository;

    @Autowired
    private InterfaceItemsRepository interfaceItemsRepository;

    @Autowired
    private ProductAttributeValueRepository productAttributeValueRepository;

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
                BigDecimal price = new BigDecimal(costItem).setScale(2, RoundingMode.DOWN);
                BigDecimal final_price = price; //price.add(BigDecimal.valueOf(150));
                product.setPrice(final_price.doubleValue());
                JumpsellerProductDto productDto = new JumpsellerProductDto(product);

                return jumpsellerClient.updateProduct(syncItem.getResponseId(), productDto)
                        .doOnSuccess(result -> {
                            System.out.println("Updated product " + syncItem.getInternalCode() + " with price " + final_price.doubleValue());
                            syncItem.setPrice(final_price.doubleValue());
                            syncProductJumpsellerRepository.save(syncItem);
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

    public Mono<Void> syncProducts(String dataAreaId) {
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts(dataAreaId);

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            System.out.println("Proccesing item id: " + syncItem.getInternalCode());
            JumpsellerProductDto product = getProductData(syncItem, dataAreaId);

            return jumpsellerClient.updateProduct(syncItem.getResponseId(), product)
                    .doOnSuccess(result -> {
                        System.out.println("Updated item id: " + syncItem.getInternalCode());
                    })
                    .doOnError(error -> {
                        System.err.println("Error updating item id: " + syncItem.getInternalCode() + " - " + error.getMessage());
                    });
        }).then().doOnSuccess(e -> {
            System.out.println("Sync items ended");
        });
    }

    public Mono<Void> syncBrands(){
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts("MSB");

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            Product product = new Product();
            product.setName(syncItem.getName());
            double price = syncItem.getPrice();
            price = Math.floor(price * 100) / 100.0;
            product.setPrice(price);

            String brand;
            String syncBrand = syncItem.getBrand() != null ? syncItem.getBrand() : "";
            switch (syncBrand){
                case "BOBCAT":
                    product.setBrand("BOBCAT");
                    brand = "BOBCAT";
                    break;
                case "DOOSAN FK":
                    product.setBrand("BOBCAT MH");
                    brand = "BOBCAT MH";
                    break;
                case "CAMSO":
                    product.setBrand("CAMSO");
                    brand = "CAMSO";
                    break;
                case "FLEXI":
                case "FELXI":
                    product.setBrand("FLEXI");
                    brand = "FLEXI";
                    break;
                case "TVH":
                case "GENERICAS":
                case "DEKA":
                case "GEN-APYMSA":
                case "MAXILEVER":
                case "APYMSA":
                case "DONALDSON":
                    product.setBrand("GENERICAS");
                    brand = "GENERICAS";
                    break;
                case "JLG":
                    product.setBrand("JLG");
                    brand = "JLG";
                    break;
                case "NISSAN":
                    product.setBrand("NISSAN");
                    brand = "NISSAN";
                    break;
                case "RALOYD":
                    product.setBrand("RAYLOD");
                    brand = "RAYLOD";
                    break;
                case "UNICARRIERS":
                    product.setBrand("UNICARRIERS");
                    brand = "UNICARRIERS";
                    break;
                default:
                    product.setBrand("OTRA");
                    brand = "OTRA";
                    break;
            }

            JumpsellerProductDto dto = new JumpsellerProductDto(product);

            return jumpsellerClient.updateProduct(syncItem.getResponseId(), dto)
                    .doOnSuccess(result -> {
                        System.out.println("Updated product " + syncItem.getInternalCode() + " with brand " + brand);
                        syncItem.setBrand(brand);
                        syncProductJumpsellerRepository.save(syncItem);
                    })
                    .doOnError(error -> {
                        System.err.println("Error updating product " + syncItem.getInternalCode() + " : " + error.getMessage());
                    });
        }).then().doOnSuccess(e -> {
            System.out.println("Product list updated ended");
        });
    }

    private JumpsellerProductDto getProductData (SyncJumpsellerProduct entity, String dataAreaId){
        HashMap<String, String> values = productUtils.getAttributes(entity, dataAreaId);

        JumpsellerProductDto product = productUtils.getProduct(values, entity);

        return product;
    }
}

