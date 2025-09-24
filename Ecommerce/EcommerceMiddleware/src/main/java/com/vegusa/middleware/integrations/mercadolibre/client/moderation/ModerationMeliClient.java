package com.vegusa.middleware.integrations.mercadolibre.client.moderation;

import com.vegusa.middleware.integrations.mercadolibre.client.MercadolibreClient;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

@Component
public class ModerationMeliClient {
    @Autowired
    private MercadolibreClient client;

    public Mono<String> getModeration(String itemId){
        return client.executeRateLimited(() ->
                client.getClient().get()
                        .uri("/moderations/last_moderation/{item-id}-ITM", itemId)
                        .retrieve()
                        .bodyToMono(String.class)
        );
    }
}
