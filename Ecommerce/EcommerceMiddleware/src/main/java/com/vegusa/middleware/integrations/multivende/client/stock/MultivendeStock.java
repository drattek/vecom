package com.vegusa.middleware.integrations.multivende.client.stock;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.Warehouse;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;

@Component
public class MultivendeStock {
    private final MultivendeClient multivendeClient;

    public MultivendeStock(MultivendeClient multivendeClient) {
        this.multivendeClient = multivendeClient;
    }

    public Mono<List<Warehouse>> getAllWarehouse(){
        Map<String, String> queryParams = Map.of(
                "merchant_id", multivendeClient.getTokenStorage().getMerchantId(),
                "page", "1"
        );
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/m/{{merchant_id}}/stores-and-warehouses/p/{{page}}", queryParams)
                        .retrieve()
                        .bodyToMono(new ParameterizedTypeReference<EntriesDto<Warehouse>>() {})
                        .map(EntriesDto::getEntries)
        );
    }

    public Mono<Warehouse> createWarehouse(Warehouse warehouse){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .post()
                        .uri("/api/m/{{merchant_id}}/stores-and-warehouses", merchantId)
                        .bodyValue(warehouse)
                        .retrieve()
                        .bodyToMono(Warehouse.class)
        );
    }
}
