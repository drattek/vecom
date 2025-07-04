package com.vegusa.middleware.integrations.mercadolibre.client.price;

import com.vegusa.middleware.integrations.mercadolibre.client.MercadolibreClient;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

@Component
public class PriceMeliClient {
    @Autowired
    private MercadolibreClient client;

    public Mono<String> getPrices(String itemId){
        return client.executeRateLimited(() ->
                client.getClient().get()
                        .uri("/items/{item_id}/prices", itemId)
                        .retrieve()
                        .bodyToMono(String.class)
        );
    }
}
