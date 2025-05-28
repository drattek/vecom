package com.vegusa.middleware.integrations.multivende.client.product;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.Size;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;

@Component
public class MultivendeSize {
    private final MultivendeClient multivendeClient;

    public MultivendeSize(MultivendeClient multivendeClient) {
        this.multivendeClient = multivendeClient;
    }

    public Mono<List<Size>> getAllSize(){
        Map<String, String> queryParams = Map.of(
                "merchant_id", multivendeClient.getTokenStorage().getMerchantId(),
                "page", "1"
        );
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/m/{{merchant_id}}/sizes/p/{{page}}", queryParams)
                        .retrieve()
                        .bodyToMono(new ParameterizedTypeReference<EntriesDto<Size>>() {})
                        .map(EntriesDto::getEntries)
        );
    }

    public Mono<Size> createSize(Size size){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .post()
                        .uri("/api/m/{{merchant_id}}/sizes", merchantId)
                        .bodyValue(size)
                        .retrieve()
                        .bodyToMono(Size.class)
        );
    }

    public Mono<Size> updateSize(Size size, String id){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .put()
                        .uri("/api/sizes/{{size_id}}", id)
                        .bodyValue(size)
                        .retrieve()
                        .bodyToMono(Size.class)
        );
    }

    public Mono<Size> getSizeById(String id){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/sizes/{{size_id}}", id)
                        .retrieve()
                        .bodyToMono(Size.class)
        );
    }
}
