package com.vegusa.middleware.integrations.multivende.client.picture;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.PictureProduct;
import com.vegusa.middleware.integrations.multivende.dto.UploadImageVersionDto;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.Map;

@Component
public class MultivendeVersionPicture {
    private final MultivendeClient multivendeClient;

    public MultivendeVersionPicture(MultivendeClient multivendeClient) {
        this.multivendeClient = multivendeClient;
    }

    public Mono<PictureProduct[]> getVersionImage(String versionId){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/product-versions/{{product_version_id}}/images", versionId)
                        .retrieve()
                        .bodyToMono(PictureProduct[].class)
        );
    }

    public Mono<PictureProduct> deletePictureVersion(String versionId){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .delete()
                        .uri("/api/product-version-images/{{product-versions-image-id}}", versionId)
                        .retrieve()
                        .bodyToMono(PictureProduct.class)
        );
    }

    public Mono<Void> uploadImageVersion(UploadImageVersionDto[] products, String pictureSet){
        Map<String, String> queryParams = Map.of(
                "merchant_id", multivendeClient.getTokenStorage().getMerchantId(),
                "product_pictures_set_id", pictureSet
        );
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .post()
                        .uri("/api/m/{{merchant_id}}/product-versions-pictures/picture-set/{{product-pictures-set-id}}/images-by-url", queryParams)
                        .bodyValue(products)
                        .retrieve()
                        .bodyToMono(Void.class)
        );
    }

    public Mono<Void> updateImageVersionPosition(UploadImageVersionDto[] products, String pictureSet){
        Map<String, String> queryParams = Map.of(
                "merchant_id", multivendeClient.getTokenStorage().getMerchantId(),
                "product_pictures_set_id", pictureSet
        );
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .put()
                        .uri("/api/m/{{merchant_id}}/product-versions-pictures/picture-set/{{product-pictures-set-id}}/update-position", queryParams)
                        .bodyValue(products)
                        .retrieve()
                        .bodyToMono(Void.class)
        );
    }
}
