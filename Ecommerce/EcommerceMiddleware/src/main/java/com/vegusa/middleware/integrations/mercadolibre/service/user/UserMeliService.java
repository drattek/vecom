package com.vegusa.middleware.integrations.mercadolibre.service.user;

import com.vegusa.middleware.integrations.mercadolibre.client.user.UserMeliClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.user.UserMeliDTO;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

@Service
public class UserMeliService {
    @Autowired
    private UserMeliClient client;

    public Mono<UserMeliDTO> getUserMe(){
        return client.getUserMe();
    }

    public Mono<String> getTestUser(){
        return client.getTestUser();
    }
}
