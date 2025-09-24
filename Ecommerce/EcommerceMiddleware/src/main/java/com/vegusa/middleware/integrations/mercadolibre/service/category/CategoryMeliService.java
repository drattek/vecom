package com.vegusa.middleware.integrations.mercadolibre.service.category;

import com.vegusa.middleware.integrations.mercadolibre.client.category.CategoryMeliClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.category.CategoryDetailMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.dto.category.CategoryMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.dto.category.PredictorMeliDTO;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

@Service
public class CategoryMeliService {
    @Autowired
    private CategoryMeliClient client;

    public Mono<PredictorMeliDTO[]> getCategoryPredictor(String query){
        return client.predictCategory(query);
    }

    public Mono<String> getCategoriesByDomain(String domain){
        return client.categoriesByDomain(domain);
    }

    public Mono<CategoryMeliDTO[]> getCategoriesBySite(){
        return client.getCategoriesBySite();
    }

    public Mono<CategoryDetailMeliDTO> getCategory(String category){
        return client.getCategory(category);
    }

    public Mono<String> getAttributes(String category){
        return client.getAttributes(category);
    }

    public Mono<String> getSaleTerms(String category){
        return client.getSaleTerms(category);
    }
}
