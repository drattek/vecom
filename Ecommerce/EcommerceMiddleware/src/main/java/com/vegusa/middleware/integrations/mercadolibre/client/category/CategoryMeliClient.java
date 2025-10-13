package com.vegusa.middleware.integrations.mercadolibre.client.category;

import com.vegusa.middleware.integrations.mercadolibre.client.MercadolibreClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.category.CategoryDetailMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.dto.category.CategoryMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.dto.category.PredictorMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.oauth.TokenStorageMeli;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

@Component
public class CategoryMeliClient {
    @Autowired
    private MercadolibreClient client;

    @Autowired
    private TokenStorageMeli tokenStorage;

    public Mono<PredictorMeliDTO[]> predictCategory(String query){
        return client.executeRateLimited(() ->
                client.getClient().get()
                        .uri(uriBuilder -> uriBuilder
                                .path("/sites/{site_id}/domain_discovery/search")
                                .queryParam("q", query)
                                .build(tokenStorage.getSiteId()))
                        .retrieve()
                        .bodyToMono(PredictorMeliDTO[].class)
        );
    }

    public Mono<String> categoriesByDomain(String domain){
        return client.executeRateLimited(() ->
                client.getClient().get()
                        .uri("/catalog_domains/{domain}/categories", domain)
                        .retrieve()
                        .bodyToMono(String.class)
        );
    }

    public Mono<CategoryMeliDTO[]> getCategoriesBySite(){
        String siteId = tokenStorage.getSiteId();
        return client.executeRateLimited(() ->
                client.getClient().get()
                        .uri("sites/{site_id}/categories", siteId)
                        .retrieve()
                        .bodyToMono(CategoryMeliDTO[].class)
        );
    }

    public Mono<CategoryDetailMeliDTO> getCategory(String category){
        return client.executeRateLimited(() ->
                client.getClient().get()
                        .uri("/categories/{category}", category)
                        .retrieve()
                        .bodyToMono(CategoryDetailMeliDTO.class)
        );
    }

    public Mono<String> getAttributes(String category){
        return client.executeRateLimited(() ->
                client.getClient().get()
                        .uri(uriBuilder -> uriBuilder
                                .path("/categories/{category_id}/attributes")
                                .build(category)
                        )
                        .retrieve()
                        .bodyToMono(String.class)
        );
    }

    public Mono<String> getSaleTerms(String category){
        return client.executeRateLimited(() ->
                client.getClient().get()
                        .uri(uriBuilder -> uriBuilder
                                .path("/categories/{category_id}/sale_terms")
                                .build(category)
                        )
                        .retrieve()
                        .bodyToMono(String.class)
        );
    }
}
