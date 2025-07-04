package com.vegusa.middleware.integrations.mercadolibre.client.user;

import com.vegusa.middleware.integrations.mercadolibre.client.MercadolibreClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.user.UserMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.oauth.TokenStorageMeli;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.Map;

@Component
public class UserMeliClient {
    @Autowired
    private MercadolibreClient client;

    @Autowired
    private TokenStorageMeli tokenStorage;

    public Mono<UserMeliDTO> getUserMe(){
        return client.executeRateLimited(() ->
                client.getClient()
                        .get()
                        .uri("/users/me")
                        .retrieve()
                        .bodyToMono(UserMeliDTO.class)
        );
    }

    public Mono<String> getTestUser(){
        String siteId = tokenStorage.getSiteId();
        Map<String, String> requestBody = Map.of("site_id", siteId);
        return client.executeRateLimited(() ->
                client.getClient().post()
                        .uri("users/test_user")
                        .bodyValue(requestBody)
                        .retrieve()
                        .bodyToMono(String.class)
        );
    }
}
