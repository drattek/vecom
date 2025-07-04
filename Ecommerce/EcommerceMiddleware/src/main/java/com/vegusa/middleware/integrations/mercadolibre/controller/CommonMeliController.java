package com.vegusa.middleware.integrations.mercadolibre.controller;

import com.vegusa.middleware.integrations.mercadolibre.dto.common.CurrencyMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.service.common.CommonMeliService;
import com.vegusa.middleware.integrations.mercadolibre.service.product.ProductMeliService;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

import java.util.HashMap;

@RestController
@RequestMapping("/msb-ecommerce-middleware/mercadolibre")
public class CommonMeliController {
    @Autowired
    private CommonMeliService commonService;

    @Autowired
    private ProductMeliService productService;

    @GetMapping(value = "/currencies")
    public Mono<CurrencyMeliDTO[]> getCurrencies(){
        return commonService.getCurrencies();
    }

    //@PostMapping(value = "/import-products")
    public Mono<ResponseEntity<String>> importProducts(@RequestBody HashMap<String, String> request){
        try {
            System.out.println("Inserting products to Database");
            String products = request.get("productList");
            JSONArray productList = new JSONObject(products).getJSONArray("content");

            commonService.importProducts(productList, "MSB").subscribe();

            return Mono.just(ResponseEntity.accepted().body("Importing products"));
        } catch (RuntimeException e){
            System.err.println("Error: " + e.getMessage());
        }

        return Mono.just(ResponseEntity.accepted().body("Sync started"));
    }

    //@PostMapping(value = "/import-category")
    public void importCategory(@RequestBody HashMap<String, String> request){
        try {
            System.out.println("Inserting categories to database");
            String categories = request.get("categoryList");
            JSONArray categoryList = new JSONObject(categories).getJSONArray("content");

            commonService.importCategories(categoryList, "MSB");
        } catch (RuntimeException e){
            System.err.println("Error: " + e.getMessage());
        }
    }

    //@PostMapping(value = "/category-correction")
    public void categoryCorrection(){
        commonService.categoryCorrection();
    }

    @GetMapping(value = "/download-images")
    public Mono<Void> downloadImages(){
        productService.downloadImages().subscribe();

        return Mono.empty();
    }
}
