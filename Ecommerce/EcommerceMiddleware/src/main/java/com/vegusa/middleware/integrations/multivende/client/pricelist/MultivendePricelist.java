package com.vegusa.middleware.integrations.multivende.client.pricelist;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.Pricelist;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;

@Component
public class MultivendePricelist {
    private final MultivendeClient multivendeClient;

    public MultivendePricelist(MultivendeClient multivendeClient) {
        this.multivendeClient = multivendeClient;
    }

    public Mono<List<Pricelist>> getAllPricelist(){
        Map<String, String> queryParams = Map.of(
                "merchant_id", multivendeClient.getTokenStorage().getMerchantId(),
                "page", "1"
        );
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/m/{merchant_id}/product-price-lists/p/{page}", queryParams)
                        .retrieve()
                        .bodyToMono(new ParameterizedTypeReference<EntriesDto<Pricelist>>() {})
                        .map(EntriesDto::getEntries)
        );
    }
}
