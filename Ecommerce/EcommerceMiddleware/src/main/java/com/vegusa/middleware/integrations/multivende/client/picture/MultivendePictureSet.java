package com.vegusa.middleware.integrations.multivende.client.picture;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.PictureSet;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.List;

@Component
public class MultivendePictureSet {
    private final MultivendeClient multivendeClient;

    public MultivendePictureSet(MultivendeClient multivendeClient) {
        this.multivendeClient = multivendeClient;
    }

    public Mono<EntriesDto<PictureSet>> getAllPictureSet(){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/m/{merchant_id}/product-picture-sets", merchantId)
                        .retrieve()
                        .bodyToMono(new ParameterizedTypeReference<EntriesDto<PictureSet>>() {})
        );
    }
}
