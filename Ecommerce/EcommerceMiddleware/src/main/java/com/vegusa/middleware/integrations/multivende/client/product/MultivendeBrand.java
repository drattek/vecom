package com.vegusa.middleware.integrations.multivende.client.product;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.Brand;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;

@Component
public class MultivendeBrand {
    private final MultivendeClient multivendeClient;

    public MultivendeBrand(MultivendeClient multivendeClient){
        this.multivendeClient = multivendeClient;
    }

    public Mono<List<Brand>> getAllBrand(){
        Map<String, String> queryParams = Map.of(
                "merchant_id", multivendeClient.getTokenStorage().getMerchantId(),
                "page", "1"
        );
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/m/{merchant_id}/brands/p/{page}", queryParams)
                        .retrieve()
                        .bodyToMono(new ParameterizedTypeReference<EntriesDto<Brand>>() {})
                        .map(EntriesDto::getEntries)
        );
    }

    public Mono<Brand> createBrand(Brand brand){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .post()
                        .uri("/api/m/{{merchant_id}}/brands", merchantId)
                        .bodyValue(brand)
                        .retrieve()
                        .bodyToMono(Brand.class)
        );
    }

    public Mono<Brand> updateBrand(Brand brand, String id){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .put()
                        .uri("/api/brands/{{brand_id}}", id)
                        .bodyValue(brand)
                        .retrieve()
                        .bodyToMono(Brand.class)
        );
    }

    public Mono<Brand> getBrandById(String id){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/brands/{{brand_id}}", id)
                        .retrieve()
                        .bodyToMono(Brand.class)
        );
    }
}
