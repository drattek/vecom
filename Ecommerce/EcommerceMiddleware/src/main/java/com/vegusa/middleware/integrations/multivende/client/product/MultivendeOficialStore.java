package com.vegusa.middleware.integrations.multivende.client.product;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.OficialStore;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.List;

@Component
public class MultivendeOficialStore {
    private final MultivendeClient multivendeClient;

    public MultivendeOficialStore(MultivendeClient multivendeClient) {
        this.multivendeClient = multivendeClient;
    }

    public Mono<List<OficialStore>> getAllOficialStore(){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/m/{{merchant_id}}/official-stores", merchantId)
                        .retrieve()
                        .bodyToMono(new ParameterizedTypeReference<EntriesDto<OficialStore>>() {})
                        .map(EntriesDto::getEntries)
        );
    }

    public Mono<OficialStore> createOficialStore(OficialStore store){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .post()
                        .uri("/api/m/{{merchant_id}}/official-stores", merchantId)
                        .bodyValue(store)
                        .retrieve()
                        .bodyToMono(OficialStore.class)
        );
    }

    public Mono<OficialStore> updateOficialStore(OficialStore store, String storeId){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .put()
                        .uri("/api/official-stores/{{official_store_id}}", storeId)
                        .bodyValue(store)
                        .retrieve()
                        .bodyToMono(OficialStore.class)
        );
    }

    public Mono<Void> deleteOficialStore(String storeId){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .delete()
                        .uri("/api/official-stores/{{official_store_id}}", storeId)
                        .retrieve()
                        .bodyToMono(Void.class)
        );
    }
}
