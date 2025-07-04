package com.vegusa.middleware.integrations.mercadolibre.client.publication;

import com.vegusa.middleware.integrations.mercadolibre.client.MercadolibreClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.publication.PublicationTypeMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.oauth.TokenStorageMeli;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

@Component
public class PublicationMeliClient {
    @Autowired
    private MercadolibreClient client;

    @Autowired
    private TokenStorageMeli tokenStorage;

    public Mono<PublicationTypeMeliDTO[]> getPublicationTypes(){
        String sideId = tokenStorage.getSiteId();
        return client.executeRateLimited(() ->
            client.getClient()
                    .get()
                    .uri("sites/{side_id}/listing_types", sideId)
                    .retrieve()
                    .bodyToMono(PublicationTypeMeliDTO[].class)
        );
    }
}
