package com.vegusa.middleware.integrations.multivende.client.product;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.Color;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;

@Component
public class MultivendeColor {
    private final MultivendeClient multivendeClient;

    public MultivendeColor(MultivendeClient multivendeClient){
        this.multivendeClient = multivendeClient;
    }

    public Mono<List<Color>> getAllColor(){
        Map<String, String> queryParams = Map.of(
                "merchant_id", multivendeClient.getTokenStorage().getMerchantId(),
                "page", "1"
        );
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/m/{{merchant_id}}/colors/p/{{page}}", queryParams)
                        .retrieve()
                        .bodyToMono(new ParameterizedTypeReference<EntriesDto<Color>>() {})
                        .map(EntriesDto::getEntries)
        );
    }

    public Mono<Color> createColor(Color color){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .post()
                        .uri("/api/m/{{merchant_id}}/colors", merchantId)
                        .bodyValue(color)
                        .retrieve()
                        .bodyToMono(Color.class)
        );
    }

    public Mono<Color> updateColor(Color color, String id){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .put()
                        .uri("/api/colors/{{color_id}}", id)
                        .bodyValue(color)
                        .retrieve()
                        .bodyToMono(Color.class)
        );
    }

    public Mono<Color> getColorById(String id){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/colors/{{color_id}}", id)
                        .retrieve()
                        .bodyToMono(Color.class)
        );
    }
}
