package com.vegusa.middleware.integrations.jumpseller.service.image;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.constants.ProductInterface;
import com.vegusa.middleware.entity.IntegrationImage;
import com.vegusa.middleware.entity.InterfaceItems;
import com.vegusa.middleware.integrations.jumpseller.client.image.ImageJumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.dto.Image;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerImageDTO;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncProductJumpsellerRepository;
import com.vegusa.middleware.repository.IntegrationImageRepository;
import com.vegusa.middleware.repository.InterfaceItemsRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.time.Instant;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

@Service
public class ImageJumpsellerService {
    @Autowired
    private ImageJumpsellerClient imageClient;

    @Autowired
    private SyncProductJumpsellerRepository syncProductJumpsellerRepository;

    @Autowired
    private InterfaceItemsRepository interfaceItemsRepository;

    @Autowired
    private IntegrationImageRepository integrationImageRepository;

    public Mono<JumpsellerImageDTO[]> getProductImages(String productId){
        return imageClient.getProductImages(productId)
                .doOnNext(result -> {
                    for (JumpsellerImageDTO jumpsellerImage : result){
                        Image image = jumpsellerImage.getImage();

                        Pattern pattern = Pattern.compile(".*?-([^-/]+\\.webp)");
                        Matcher matcher = pattern.matcher(image.getUrl());

                        if (matcher.find()){
                            String fileName = matcher.group(1);
                            System.out.println("Filename: " + fileName);
                        }
                    }
                });
    }

    public Mono<Void> downloadImages(){
        String basePath = "https://vconstorage2.blob.core.windows.net/veg-ecomm-products/";
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts(DataArea.MSB.name());

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            return imageClient.getProductImages(syncItem.getResponseId().toString())
                    .doOnSuccess(result -> {
                        for (JumpsellerImageDTO jumpsellerImage : result){
                            Image image = jumpsellerImage.getImage();
                            IntegrationImage syncImage = new IntegrationImage();
                            InterfaceItems localProduct = interfaceItemsRepository.getInterfaceProduct(
                                    syncItem.getInternalCode(),
                                    ProductInterface.DYN.name(),
                                    DataArea.MSB.name()
                            );

                            syncImage.setProductId(localProduct.getId());
                            syncImage.setInternalCode(syncItem.getInternalCode());
                            syncImage.setProductExternalId(syncItem.getResponseId().toString());
                            syncImage.setExternalId(image.getId().toString());
                            syncImage.setExternalUrl(image.getUrl());
                            syncImage.setPosition(image.getPosition());
                            syncImage.setDataAreaId(DataArea.MSB.name());
                            syncImage.setIntegrationName(IntegrationType.JUMPSELLER.name());
                            syncImage.setIntegrationParameterId(2L);
                            syncImage.setCompanyId(1L);
                            syncImage.setCreatedAt(Instant.now());
                            syncImage.setUpdatedAt(Instant.now());

                            //Pattern pattern = Pattern.compile(".*?-([^-/]+\\.webp)");
                            //Pattern pattern = Pattern.compile(".*?-([^-/]+\\.(webp|png|jpe|jpeg))");
                            Pattern pattern = Pattern.compile(".*?(MSB[\\w\\-]+\\.(webp|png|jpe|jpeg))", Pattern.CASE_INSENSITIVE);
                            Matcher matcher = pattern.matcher(image.getUrl());

                            if (matcher.find()){
                                String fileName = matcher.group(1);
                                syncImage.setFileName(normalizeFileName(fileName));
                                syncImage.setUrl(basePath + normalizeFileName(fileName));
                            }

                            integrationImageRepository.save(syncImage);
                        }
                        System.out.println("Download images: " + syncItem.getInternalCode());
                    })
                    .doOnError(error -> System.err.println("Error: " + error.getMessage()));
        }).then().doOnSuccess(e -> System.out.println("Download completed"));
    }

    private String normalizeFileName(String fileName) {
        int dotIndex = fileName.lastIndexOf('.');
        if (dotIndex == -1) {
            return fileName.toUpperCase();
        }

        String namePart = fileName.substring(0, dotIndex);
        String extensionPart = fileName.substring(dotIndex + 1);

        return namePart.toUpperCase() + "." + extensionPart.toLowerCase();
    }
}
