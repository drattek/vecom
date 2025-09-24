package com.vegusa.middleware.integrations.mercadolibre.service.moderation;

import com.vegusa.middleware.integrations.mercadolibre.client.moderation.ModerationMeliClient;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

@Service
public class ModerationMeliService {
    @Autowired
    private ModerationMeliClient client;

    public Mono<String> getModeration(String itemId){
        return client.getModeration(itemId);
    }
}
