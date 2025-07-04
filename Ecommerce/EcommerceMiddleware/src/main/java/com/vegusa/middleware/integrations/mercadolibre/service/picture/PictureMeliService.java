package com.vegusa.middleware.integrations.mercadolibre.service.picture;

import com.vegusa.middleware.integrations.mercadolibre.client.picture.PictureMeliClient;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

@Service
public class PictureMeliService {
    @Autowired
    private PictureMeliClient client;

    public Mono<String> getErrors(String pictureId){
        return client.getErrors(pictureId);
    }
}
