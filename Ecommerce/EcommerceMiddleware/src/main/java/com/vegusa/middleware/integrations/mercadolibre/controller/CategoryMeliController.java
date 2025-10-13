package com.vegusa.middleware.integrations.mercadolibre.controller;

import com.vegusa.middleware.integrations.mercadolibre.dto.category.CategoryDetailMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.dto.category.CategoryMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.dto.category.PredictorMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.service.category.CategoryMeliService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

import java.util.HashMap;

@RestController
@RequestMapping("/msb-ecommerce-middleware/mercadolibre")
public class CategoryMeliController {
    @Autowired
    private CategoryMeliService categoryService;

    @GetMapping(value = "/category-predictor")
    public Mono<PredictorMeliDTO[]> getCategoryPredictor(@RequestParam("title") String title){
        if (title.isBlank()){
            return Mono.empty();
        }
        return categoryService.getCategoryPredictor(title);
    }

    @GetMapping(value = "/category-domain")
    public Mono<String> getCategoryByDomain(@RequestParam("domain") String domain){
        if (domain.isBlank()){
            return Mono.empty();
        }
        return categoryService.getCategoriesByDomain(domain);
    }

    @GetMapping(value = "/category-site")
    public Mono<CategoryMeliDTO[]> getCategoryBySite(){
        return categoryService.getCategoriesBySite();
    }

    @GetMapping(value = "/category")
    public Mono<CategoryDetailMeliDTO> getCategory(@RequestParam("category_id") String categoryId){
        return categoryService.getCategory(categoryId);
    }

    @PostMapping(value = "/get-attributes")
    public Mono<String> getAttributes(@RequestBody HashMap<String, String> request){
        String categoryId = request.get("category_id");
        return categoryService.getAttributes(categoryId);
    }

    @PostMapping(value = "/get-sale-terms")
    public Mono<String> getSaleTerms(@RequestBody HashMap<String, String> request){
        String categoryId = request.get("category_id");
        return categoryService.getSaleTerms(categoryId);
    }
}
