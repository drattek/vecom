package com.vegusa.middleware.integrations.mercadolibre.service.product;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.constants.ProductInterface;
import com.vegusa.middleware.entity.IntegrationImage;
import com.vegusa.middleware.entity.InterfaceItems;
import com.vegusa.middleware.integrations.mercadolibre.client.product.ProductMeliClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.product.PictureMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.dto.product.ProductMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.entity.SyncItemMeli;
import com.vegusa.middleware.integrations.mercadolibre.repository.SyncItemMeliRepository;
import com.vegusa.middleware.repository.IntegrationImageRepository;
import com.vegusa.middleware.repository.InterfaceItemsRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.time.Instant;
import java.util.Map;

@Service
public class ProductMeliService {
    @Autowired
    private ProductMeliClient client;

    @Autowired
    private SyncItemMeliRepository syncItemMeliRepository;

    @Autowired
    private InterfaceItemsRepository interfaceItemsRepository;

    @Autowired
    private IntegrationImageRepository integrationImageRepository;

    public Mono<String> getProducts(){
        return client.getProducts();
    }

    public Mono<ProductMeliDTO> getProduct(String itemId){
        return client.getProduct(itemId);
    }

    public Mono<String> createProduct(Map<String, Object> data){
        return client.createProduct(data);
    }

    public Mono<Void> downloadProducts(){
        SyncItemMeli[] syncItems = syncItemMeliRepository.getItems("MSB");

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            return client.getProduct(syncItem.getResponseId())
                    .doOnSuccess(result -> {
                        syncItem.setName(result.getTitle());
                        syncItem.setPrice(result.getPrice());
                        syncItem.setAvailable(result.getAvailableQuantity());
                        syncItem.setUserProductId(result.getUserProductId());
                        syncItem.setCurrencyId(result.getCurrencyId());
                        syncItem.setPermalink(result.getPermalink());
                        syncItem.setStatus(result.getStatus());
                        syncItem.setDomainId(result.getDomainId());
                        syncItem.setChannels(String.join(",", result.getChannels()));

                        syncItemMeliRepository.save(syncItem);
                        System.out.println("Product downloaded: " + syncItem.getInternalCode());
                    })
                    .doOnError(error -> {
                        System.err.println("Error: " + error.getMessage());
                    });
        }).then().doOnSuccess(e -> {
            System.out.println("Download products completed");
        });
    }

    public Mono<Void> downloadImages(){
        String basePath = "https://vconstorage2.blob.core.windows.net/veg-ecomm-products/";
        SyncItemMeli[] syncItems = syncItemMeliRepository.getItems(DataArea.MSB.name());
        System.out.println("Start downloading images");

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            return client.getProduct(syncItem.getResponseId())
                    .doOnSuccess(result -> {
                        PictureMeliDTO[] images = result.getPictures();
                        InterfaceItems localProduct = interfaceItemsRepository.getInterfaceProduct(
                                syncItem.getInternalCode(),
                                ProductInterface.DYN.name(),
                                DataArea.MSB.name()
                        );
                        int position = 0;
                        for (PictureMeliDTO image : images){
                            IntegrationImage syncImage = new IntegrationImage();

                            syncImage.setProductId(localProduct.getId());
                            syncImage.setInternalCode(syncItem.getInternalCode());
                            syncImage.setProductExternalId(syncItem.getResponseId());
                            syncImage.setExternalId(image.getId());
                            syncImage.setExternalUrl(image.getSecureUrl());
                            syncImage.setPosition((long) position);
                            syncImage.setDataAreaId(DataArea.MSB.name());
                            syncImage.setIntegrationName(IntegrationType.MERCADO_LIBRE.name());
                            syncImage.setIntegrationParameterId(1L);
                            syncImage.setCompanyId(1L);
                            syncImage.setCreatedAt(Instant.now());
                            syncImage.setUpdatedAt(Instant.now());

                            String fileName = integrationImageRepository.getFileName(syncItem.getInternalCode(), position);
                            if (fileName != null && !fileName.isEmpty()){
                                syncImage.setFileName(fileName);
                                syncImage.setUrl(basePath + fileName);
                            }

                            integrationImageRepository.save(syncImage);
                            position = position + 1;
                        }
                        System.out.println("Image item " + syncItem.getInternalCode() + " downloaded");
                    })
                    .doOnError(error -> System.err.println("Error downloading image " + syncItem.getInternalCode()));
        }).then().doOnSuccess(e -> System.out.println("Images Mercado Libre downloaded"));
    }

    public Mono<Void> syncImages(){
        PictureMeliDTO picture = new PictureMeliDTO();
        picture.setSource("https://images.jumpseller.com/store/vegusa/28916953/im-prod-products-images/56a62fdd-7db9-4081-ac79-88fe4bc28e23-msb-0000044_0490100100w_ai_1.png?1742609769");

        PictureMeliDTO picture2 = new PictureMeliDTO();
        picture2.setSource("https://images.jumpseller.com/store/vegusa/28916953/im-prod-products-images/a1bdcce8-5db9-4631-a7ed-0e0cbc2d6329-msb-0000044_0490100100w_ai_2.png?1742609772");

        PictureMeliDTO[] pictures = { picture, picture2 };

        ProductMeliDTO product = new ProductMeliDTO();
        product.setPictures(pictures);

        client.updateProduct(product, "MLM3478627526")
                .doOnSuccess(result -> {
                    System.out.println("Product updated");
                })
                .subscribe();

        return Mono.empty();
    }
}
