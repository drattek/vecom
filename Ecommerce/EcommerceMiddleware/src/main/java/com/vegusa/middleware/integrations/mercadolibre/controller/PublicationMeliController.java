package com.vegusa.middleware.integrations.mercadolibre.controller;

import com.vegusa.middleware.integrations.mercadolibre.dto.publication.PublicationTypeMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.service.publication.PublicationMeliService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

@RestController
@RequestMapping("/msb-ecommerce-middleware/mercadolibre")
public class PublicationMeliController {
    @Autowired
    private PublicationMeliService publicationService;

    @GetMapping("/publication-types")
    public Mono<PublicationTypeMeliDTO[]> getPublicationTypes(){
        return publicationService.getPublicationTypes();
    }
}
