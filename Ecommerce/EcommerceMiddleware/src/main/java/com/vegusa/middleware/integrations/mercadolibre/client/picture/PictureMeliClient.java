package com.vegusa.middleware.integrations.mercadolibre.client.picture;

import com.vegusa.middleware.integrations.mercadolibre.client.MercadolibreClient;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

@Component
public class PictureMeliClient {
    @Autowired
    private MercadolibreClient client;

    public Mono<String> getErrors(String pictureId){
        return client.executeRateLimited(() ->
                client.getClient().get()
                        .uri("/pictures/{picture_id}/errors", pictureId)
                        .retrieve()
                        .bodyToMono(String.class)
        );
    }
}
