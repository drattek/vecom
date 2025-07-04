package com.vegusa.middleware.integrations.mercadolibre.service.product;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.constants.ProductInterface;
import com.vegusa.middleware.dto.ProductInfo;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.integrations.mercadolibre.client.product.ProductMeliClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.category.AttributeMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.dto.product.PictureMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.dto.product.ProductMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.entity.SyncItemMeli;
import com.vegusa.middleware.integrations.mercadolibre.repository.SyncItemMeliRepository;
import com.vegusa.middleware.repository.IntegrationCategoryRepository;
import com.vegusa.middleware.repository.IntegrationImageRepository;
import com.vegusa.middleware.repository.InterfaceItemsRepository;
import com.vegusa.middleware.utils.SyncUtils;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.time.Instant;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

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

    @Autowired
    private SyncUtils syncUtils;

    @Autowired
    private IntegrationCategoryRepository categoryRepository;

    public Mono<String> getProducts(){
        return client.getProducts();
    }

    public Mono<ProductMeliDTO> getProduct(String itemId){
        return client.getProduct(itemId);
    }

//    public Mono<String> createProduct(Map<String, Object> data){
//        return client.createProduct(data);
//    }

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

    // CREATE NEW PRODUCT FROM SYNC ITEMS MULTISTORE

    public Mono<Void> syncProduct(Map<String, ProductInfo> listProducts){
        Map<Long, IntegrationCategory> categories = categoryRepository.findByIntegrationName(IntegrationType.MERCADO_LIBRE.name())
                .map(list -> list.stream().collect(Collectors.toMap(
                        IntegrationCategory::getCategoryId,
                        category -> category
                )))
                .orElseGet(HashMap::new);

        Map<String, ProductMeliDTO> products = new HashMap<>();
        for (Map.Entry<String, ProductInfo> entry : listProducts.entrySet()){
            String itemId = entry.getKey();
            ProductInfo info = entry.getValue();
            Products baseProduct = info.getProduct();
            ProductCategories baseCategory = info.getCategory();
            List<ProductImage> images = info.getImages();

            if (syncItemMeliRepository.findByInternalCode(itemId).isEmpty()){
                ProductMeliDTO product = new ProductMeliDTO();
                product.setPrice(info.getPrice());
                product.setAvailableQuantity(info.getStock().intValue());
                product.setCurrencyId("MXN");
                product.setCondition("new");
                product.setBuyingMode("buy_it_now");
                product.setListingTypeId("gold_pro");

                IntegrationCategory syncCategory = categories.get(baseCategory.getCategoryRefRecId());
                product.setCategoryId(syncCategory.getExternalId());

                String name = syncUtils.getName(baseProduct, baseProduct.getShortDescription());
                product.setTitle(name);

                List<AttributeMeliDTO> attributes = getAttributesMeli(baseProduct);
                product.setAttributes(attributes.toArray(AttributeMeliDTO[]::new));

                List<PictureMeliDTO> pictures = new ArrayList<>();
                for (ProductImage image : images){
                    PictureMeliDTO newImage = new PictureMeliDTO();
                    newImage.setSource(image.getImageUrl());
                    pictures.add(newImage);
                }
                product.setPictures(pictures.toArray(PictureMeliDTO[]::new));

                products.put(itemId, product);
            }
        }

        return Flux.fromIterable(products.entrySet()).flatMap(productEntry -> {
            String itemId = productEntry.getKey();
            ProductMeliDTO product = productEntry.getValue();

            return client.createProduct(product)
                    .doOnSuccess(response -> {
                        ProductInfo info = listProducts.get(itemId);
                        Products tmpProduct = info.getProduct();

                        SyncItemMeli newProduct = new SyncItemMeli();
                        newProduct.setResponseId(response.getId());
                        newProduct.setInternalCode(itemId);
                        newProduct.setCode(tmpProduct.getPartNumber());
                        newProduct.setName(response.getTitle());
                        newProduct.setPrice(response.getPrice());
                        newProduct.setAvailable(response.getAvailableQuantity());
                        newProduct.setCategoryId(response.getCategoryId());
                        newProduct.setUserProductId(response.getUserProductId());
                        newProduct.setCurrencyId(response.getCurrencyId());
                        newProduct.setPermalink(response.getPermalink());
                        newProduct.setStatus(response.getStatus());
                        newProduct.setDomainId(response.getDomainId());
                        String channels = String.join(",", response.getChannels());
                        newProduct.setChannels(channels);
                        newProduct.setDataAreaId(DataArea.MSB.name());
                        newProduct.setCompanyRefRecId(1L);

                        syncItemMeliRepository.save(newProduct);
                        System.out.println("Item " + itemId + " saved on Mercado Libre");
                    });
        }).then().doOnSuccess(e -> System.out.println("Synchronization Mercado Libre completed"));
    }

    private List<AttributeMeliDTO> getAttributesMeli(Products product){
        List<AttributeMeliDTO> attributes = new ArrayList<>();

        // Brand attribute
        AttributeMeliDTO brand = new AttributeMeliDTO();
        brand.setId("BRAND");
        brand.setValueName(product.getBrand());
        attributes.add(brand);

        // Part number
        AttributeMeliDTO partNumber = new AttributeMeliDTO();
        partNumber.setId("PART_NUMBER");
        partNumber.setValueName(product.getPartNumber());
        attributes.add(partNumber);

        // Item condition
        AttributeMeliDTO itemCondition = new AttributeMeliDTO();
        itemCondition.setId("ITEM_CONDITION");
        itemCondition.setValueId("2230284");
        attributes.add(itemCondition);

        // Model
        AttributeMeliDTO model = new AttributeMeliDTO();
        model.setId("MODEL");
        model.setValueName(product.getPartNumber());
        attributes.add(model);

        // Seller sku
        AttributeMeliDTO sellerSku = new AttributeMeliDTO();
        sellerSku.setId("SELLER_SKU");
        sellerSku.setValueName(product.getPartNumber());
        attributes.add(sellerSku);

        return attributes;
    }
}
