package com.vegusa.middleware.integrations.mercadolibre.service.publication;

import com.vegusa.middleware.integrations.mercadolibre.client.publication.PublicationMeliClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.publication.PublicationTypeMeliDTO;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

@Service
public class PublicationMeliService {
    @Autowired
    private PublicationMeliClient client;

    public Mono<PublicationTypeMeliDTO[]> getPublicationTypes(){
        return client.getPublicationTypes();
    }
}
