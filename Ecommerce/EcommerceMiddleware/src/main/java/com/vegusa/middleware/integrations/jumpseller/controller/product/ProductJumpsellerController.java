package com.vegusa.middleware.integrations.jumpseller.controller.product;

import com.vegusa.middleware.integrations.jumpseller.service.product.ProductJumpsellerService;
import com.vegusa.middleware.utils.MWUtils;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

import java.util.HashMap;

@RestController
@RequestMapping("msb-ecommerce-middleware/jumpseller")
public class ProductJumpsellerController {

    @Autowired
    private ProductJumpsellerService productJumpsellerService;

    @Autowired
    public ProductJumpsellerController(){}

    @GetMapping(value = "/get-products")
    public Mono<ResponseEntity<String>> getProducts(){
        try {
            System.out.println("Get all products");
            productJumpsellerService.getAllProducts().subscribe();

            return Mono.just(ResponseEntity.accepted().body("Getting all products"));
        } catch (RuntimeException e){
            System.err.println("Error getting all products");
        }
        return null;
    }

    @PostMapping(value = "/sync-prices")
    public Mono<ResponseEntity<String>> updatePricesJumpseller (@RequestBody HashMap<String, String> request){
        try {
            System.out.println("Start updating prices");
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId")),
                    products = MWUtils.bodyValidation(request.get("productList")); // Read the array of products uploaded
            JSONArray productsList = new JSONObject(products).getJSONArray("content"); // Transform the productlist to JsonArray
            productJumpsellerService.updatePrices(productsList, dataAreaId).subscribe();

            return Mono.just(ResponseEntity.accepted().body("Update started"));
        } catch (RuntimeException e) {
            System.err.println(e.getMessage());
        }
        return null;
    }

    @PostMapping(value = "/sync-brands")
    public Mono<ResponseEntity<String>> updateBrands(){
        try {
            System.out.println("Start updating brands");
            productJumpsellerService.syncBrands().subscribe();

            return Mono.just(ResponseEntity.accepted().body("Update started"));
        } catch (RuntimeException e){
            System.err.println(e.getMessage());
        }
        return null;
    }

    @PostMapping(value = "/sync-products")
    public Mono<ResponseEntity<String>> synProductJumpseller (@RequestBody HashMap<String, String> request){
        try {
            System.out.println("Start sync products");
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId"));
            productJumpsellerService.syncProducts(dataAreaId).subscribe();

            return Mono.just(ResponseEntity.accepted().body("Syn started"));
        } catch (RuntimeException e) {
            System.err.println(e.getMessage());
        }
        return null;
    }

    @PostMapping(value = "/update-title")
    public Mono<Void> updateTitles(){
        productJumpsellerService.changeTitle().subscribe();

        return Mono.empty();
    }
}
