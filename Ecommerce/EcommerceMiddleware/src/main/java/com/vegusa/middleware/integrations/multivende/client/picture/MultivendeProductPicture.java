package com.vegusa.middleware.integrations.multivende.client.picture;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.PictureProduct;
import com.vegusa.middleware.integrations.multivende.dto.UploadImageDto;
import com.vegusa.middleware.integrations.multivende.dto.UploadImageResponse;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.Map;

@Component
public class MultivendeProductPicture {
    private final MultivendeClient multivendeClient;

    public MultivendeProductPicture(MultivendeClient multivendeClient) {
        this.multivendeClient = multivendeClient;
    }

    public Mono<PictureProduct[]> getProductImages(String id){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/products/{{product_id}}/images", id)
                        .retrieve()
                        .bodyToMono(PictureProduct[].class)
        );
    }

    public Mono<PictureProduct> deleteImage(String id){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .delete()
                        .uri("/api/product-images/{{product-image-id}}", id)
                        .retrieve()
                        .bodyToMono(PictureProduct.class)
        );
    }

    public Mono<UploadImageResponse[]> uploadImageByUrl(UploadImageDto[] products, String pictureSet){
        Map<String, String> queryParams = Map.of(
                "merchant_id", multivendeClient.getTokenStorage().getMerchantId(),
                "product_pictures_set_id", pictureSet
        );
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .post()
                        .uri("/api/m/{{merchant_id}}/product-pictures/picture-set/{{product-pictures-set-id}}/images-by-url", queryParams)
                        .bodyValue(products)
                        .retrieve()
                        .bodyToMono(UploadImageResponse[].class)
        );
    }

    public Mono<Void> updateImagePosition(UploadImageDto[] products, String pictureSet){
        Map<String, String> queryParams = Map.of(
                "merchant_id", multivendeClient.getTokenStorage().getMerchantId(),
                "product_pictures_set_id", pictureSet
        );
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .put()
                        .uri("/api/m/{{merchant_id}}/product-pictures/picture-set/{{product-pictures-set-id}}/update-position", queryParams)
                        .bodyValue(products)
                        .retrieve()
                        .bodyToMono(Void.class)
        );
    }
}
