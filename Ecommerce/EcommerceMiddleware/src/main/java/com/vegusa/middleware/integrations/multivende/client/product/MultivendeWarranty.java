package com.vegusa.middleware.integrations.multivende.client.product;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.Warranty;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;

@Component
public class MultivendeWarranty {
    private final MultivendeClient multivendeClient;

    public MultivendeWarranty(MultivendeClient multivendeClient) {
        this.multivendeClient = multivendeClient;
    }

    public Mono<List<Warranty>> getAllWarranty(){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/m/{{merchant_id}}/warranties", merchantId)
                        .retrieve()
                        .bodyToMono(new ParameterizedTypeReference<EntriesDto<Warranty>>() {})
                        .map(EntriesDto::getEntries)
        );
    }

    public Mono<Warranty> createWarranty(Warranty warranty){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .post()
                        .uri("/api/m/{{merchant_id}}/warranties", merchantId)
                        .bodyValue(warranty)
                        .retrieve()
                        .bodyToMono(Warranty.class)
        );
    }

    public Mono<Warranty> updateWarranty(Warranty warranty, String warrantyId){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .put()
                        .uri("/api/warranties/{{warranties_id}}", warrantyId)
                        .bodyValue(warranty)
                        .retrieve()
                        .bodyToMono(Warranty.class)
        );
    }

    public Mono<Void> deleteWarranty(String warrantyId){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .delete()
                        .uri("/api/warranties/{{warranties_id}}", warrantyId)
                        .retrieve()
                        .bodyToMono(Void.class)
        );
    }
}
