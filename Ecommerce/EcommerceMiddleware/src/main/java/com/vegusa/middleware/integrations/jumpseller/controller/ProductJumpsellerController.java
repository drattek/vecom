package com.vegusa.middleware.integrations.jumpseller.controller;

import com.vegusa.middleware.integrations.jumpseller.service.JumpsellerProductService;
import com.vegusa.middleware.utils.MWUtils;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

import java.util.HashMap;

@RestController
@RequestMapping("msb-ecommerce-middleware/jumpseller")
public class ProductJumpsellerController {

    @Autowired
    JumpsellerProductService jumpsellerProductService;

    @Autowired
    public ProductJumpsellerController(){}

    @PostMapping(value = "/sync-prices")
    public Mono<ResponseEntity<String>> updatePricesJumpseller (@RequestBody HashMap<String, String> request){
        try {
            System.out.println("Start updating prices");
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId")),
                    products = MWUtils.bodyValidation(request.get("productList")); // Read the array of products uploaded
            JSONArray productsList = new JSONObject(products).getJSONArray("content"); // Transform the productlist to JsonArray
            jumpsellerProductService.updatePrices(productsList, dataAreaId).subscribe();

            return Mono.just(ResponseEntity.accepted().body("Update started"));
        } catch (RuntimeException e) {
            System.err.println(e.getMessage());
        }
        return null;
    }

    @PostMapping(value = "/sync-products")
    public Mono<ResponseEntity<String>> synProductJumpseller (@RequestBody HashMap<String, String> request){
        try {
            System.out.println("Start sync products");
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId"));
        } catch (RuntimeException e) {
            System.err.println(e.getMessage());
        }
        return null;
    }
}
