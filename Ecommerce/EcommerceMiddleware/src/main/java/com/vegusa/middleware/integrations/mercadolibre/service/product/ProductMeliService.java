package com.vegusa.middleware.integrations.mercadolibre.service.product;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.constants.PriceParameter;
import com.vegusa.middleware.constants.ProductInterface;
import com.vegusa.middleware.dto.ProductInfo;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.integrations.mercadolibre.client.product.ProductMeliClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.category.AttributeMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.dto.product.DescriptionMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.dto.product.PictureMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.dto.product.ProductMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.dto.product.ShippingMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.entity.SyncItemMeli;
import com.vegusa.middleware.integrations.mercadolibre.repository.SyncItemMeliRepository;
import com.vegusa.middleware.integrations.mercadolibre.service.price.PriceMeliService;
import com.vegusa.middleware.repository.IntegrationCategoryRepository;
import com.vegusa.middleware.repository.IntegrationImageRepository;
import com.vegusa.middleware.repository.InterfaceItemsRepository;
import com.vegusa.middleware.repository.ProductAttributeValuesRepository;
import com.vegusa.middleware.utils.SyncUtils;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.*;
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

    @Autowired
    private PriceMeliService priceMeliService;

    @Autowired
    private ProductAttributeValuesRepository attributeValuesRepository;

    public Mono<String> getProducts(){
        return client.getProducts();
    }

    public Mono<ProductMeliDTO> getProduct(String itemId){
        return client.getProduct(itemId);
    }

    public Mono<String> getProduct2(String itemId){
        return client.getProduct2(itemId);
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

    public Mono<Void> updateShipping(){
        List<SyncItemMeli> syncItems = syncItemMeliRepository.getNewItems();

        return Flux.fromArray(syncItems.toArray(SyncItemMeli[]::new)).flatMap(syncItem -> {
            ProductMeliDTO product = new ProductMeliDTO();
            ShippingMeliDTO shipping = new ShippingMeliDTO();
            shipping.setMode("me2");
            shipping.setFreeShipping(true);
            shipping.setLocalPickUp(false);
            shipping.setFreeMethods(new String[0]);

            product.setShipping(shipping);
            product.setPrice(syncItem.getPrice());

            return client.updateProduct(product, syncItem.getResponseId())
                    .doOnSuccess(result -> {
                        System.out.println("Updated item: " + syncItem.getInternalCode());
                        syncItem.setStatus(result.getStatus());
                        syncItemMeliRepository.save(syncItem);
                    })
                    .doOnError(error -> System.err.println("Error item: " + syncItem.getInternalCode()));
        }).then().doOnSuccess(e -> System.out.println("Update completed"));
    }

    public Mono<Void> updateDescription(){
        List<SyncItemMeli> syncItems = syncItemMeliRepository.getNewItems();

        return Flux.fromArray(syncItems.toArray(SyncItemMeli[]::new)).flatMap(syncItem -> {
            ProductAttributeValues description = attributeValuesRepository.getAttribute(syncItem.getInternalCode(), "META_DESCRIPTION")
                    .orElseGet(() -> {
                        Optional<ProductAttributeValues> tmp = attributeValuesRepository.getAttribute(syncItem.getInternalCode(), "SHORT_DESCRIPTION");
                        return tmp.orElse(null);
                    });
            ProductAttributeValues crossReferences = attributeValuesRepository.getAttribute(syncItem.getInternalCode(), "CROSS_REFERENCES")
                    .orElseGet(() -> null);
            if (description != null){
                DescriptionMeliDTO dto = new DescriptionMeliDTO();
                if (Objects.equals(description.getProductAttributeId(), "META_DESCRIPTION")){
                    dto.setPlainText(description.getValue());
                } else {
                    String cross = crossReferences != null ? crossReferences.getValue() : syncItem.getInternalCode();
                    String tmp = "-- TIENDA VEGUSA MAQUINARIA, DISTRUIBIDOR AUTORIZADO UNICARRIERS, BOBCAT, JLG, FLEXI. -- " + description.getValue() + ". Equivalente con: " + cross;
                    dto.setPlainText(tmp);
                }
                return client.createDescription(syncItem.getResponseId(), dto)
                        .doOnSuccess(result -> {
                            System.out.println(result);
                            syncItem.setDescription(dto.getPlainText());
                            syncItemMeliRepository.save(syncItem);
                        });
            }
            return Mono.empty();
        }).then().doOnSuccess(e -> System.out.println("Descriptions concluded"));
    }

    public Mono<Void> syncImages(){
        PictureMeliDTO picture = new PictureMeliDTO();
        picture.setSource("https://vconstorage2.blob.core.windows.net/veg-ecomm-products/MSB-0000044_0490100100W_AI_1.png");

        PictureMeliDTO picture2 = new PictureMeliDTO();
        //picture2.setId("975290-MLM87351272233_072025");
        picture2.setSource("https://vconstorage2.blob.core.windows.net/veg-ecomm-products/MSB-0000044_0490100100W_AI_2.png");

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

            Optional<SyncItemMeli> syncItemOptional = syncItemMeliRepository.findByInternalCode(itemId);
            if (syncItemOptional.isEmpty()){
                ProductMeliDTO product = new ProductMeliDTO();
                product.setPrice(priceMeliService.getPrice(info.getPrice(), itemId));
                product.setAvailableQuantity(info.getStock().intValue());
                product.setCurrencyId("MXN");
                product.setCondition("new");
                product.setBuyingMode("buy_it_now");
                product.setListingTypeId("gold_pro");

                IntegrationCategory syncCategory = categories.get(baseCategory.getCategoryRefRecId());
                product.setCategoryId(syncCategory.getExternalId());

                String name = syncUtils.getName(baseProduct, baseProduct.getShortDescription());
                product.setFamilyName(name);

                List<AttributeMeliDTO> attributes = getAttributesMeli(baseProduct);
                product.setAttributes(attributes.toArray(AttributeMeliDTO[]::new));

                List<PictureMeliDTO> pictures = new ArrayList<>();
                for (ProductImage image : images){
                    PictureMeliDTO newImage = new PictureMeliDTO();
                    newImage.setSource(image.getImageUrl());
                    pictures.add(newImage);
                }
                product.setPictures(pictures.toArray(PictureMeliDTO[]::new));

                ShippingMeliDTO shipping = new ShippingMeliDTO();
                shipping.setMode("me2");
                shipping.setFreeShipping(true);
                shipping.setLocalPickUp(false);
                shipping.setFreeMethods(new String[0]);
                product.setShipping(shipping);

                products.put(itemId, product);
            } else {
                SyncItemMeli syncItem = syncItemOptional.get();
                if (!Objects.equals(syncItem.getStatus(), "under_review")){
                    ProductMeliDTO product = new ProductMeliDTO();
                    product.setPrice(priceMeliService.getPrice(info.getPrice(), itemId));
                    product.setAvailableQuantity(info.getStock().intValue());

                    List<PictureMeliDTO> pictures = new ArrayList<>();
                    for (ProductImage image : images){
                        PictureMeliDTO newImage = new PictureMeliDTO();
                        newImage.setSource(image.getImageUrl());
                        pictures.add(newImage);
                    }
                    product.setPictures(pictures.toArray(PictureMeliDTO[]::new));
                    client.updateProduct(product, syncItem.getResponseId())
                            .doOnSuccess(result -> {
                                syncItem.setPrice(result.getPrice());
                                syncItem.setAvailable(result.getAvailableQuantity());
                                syncItemMeliRepository.save(syncItem);
                            }).subscribe();
                }
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

                        newProduct = syncItemMeliRepository.save(newProduct);
                        System.out.println("Item " + itemId + " saved on Mercado Libre");
                        createDescription(listProducts.get(itemId), response.getId(), newProduct).subscribe();
                    })
                    .doOnError(error -> System.err.println("Error meli: " + error.getMessage()));
        }).then().doOnSuccess(e -> System.out.println("Synchronization Mercado Libre completed"));
    }

    private Mono<Void> createDescription(ProductInfo info, String itemId, SyncItemMeli syncItem){
        Products product = info.getProduct();
        DescriptionMeliDTO tmp = new DescriptionMeliDTO();
        if (product.getMetaDescription().isEmpty()){
            String tmpDescription = "-- TIENDA VEGUSA MAQUINARIA, DISTRUIBIDOR AUTORIZADO UNICARRIERS, BOBCAT, JLG, FLEXI. -- " + product.getShortDescription() + ". Equivalente con: " + product.getCrossReferences();
            tmp.setPlainText(tmpDescription);
        } else {
            tmp.setPlainText(product.getMetaDescription());
        }

        return client.createDescription(itemId, tmp)
                .doOnSuccess(result -> {
                    syncItem.setDescription(tmp.getPlainText());
                    syncItemMeliRepository.save(syncItem);
                }).then();
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

    // User products

    public Mono<String> getUserProduct(String itemId){
        return client.getUserProduct(itemId);
    }
}
