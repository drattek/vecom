package com.vegusa.middleware.integrations.mercadolibre.controller;

import com.vegusa.middleware.integrations.mercadolibre.dto.product.ProductMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.service.product.ProductMeliService;
import com.vegusa.middleware.integrations.mercadolibre.service.stock.StockMeliService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

import java.util.Map;

@RestController
@RequestMapping("/msb-ecommerce-middleware/mercadolibre")
public class ProductMeliController {
    @Autowired
    private ProductMeliService productService;

    @Autowired
    private StockMeliService stockService;

    @PostMapping(value = "/download-products")
    public Mono<ResponseEntity<String>> downloadProducts(){
        productService.downloadProducts().subscribe();

        return Mono.just(ResponseEntity.accepted().body("Download started"));
    }

    @GetMapping(value = "/products")
    public Mono<Void> getProducts(@RequestParam Map<String, String> data){
        String scroll = data.get("scroll_id");
        productService.getProducts(scroll);

        return Mono.empty();
    }

    @PostMapping(value = "/resync-meli-products")
    public void resyncMeliProducts(){
        productService.resyncProducts();
    }

    @GetMapping(value = "/product")
    public Mono<ProductMeliDTO> getProduct(@RequestParam("item_id") String itemId){
        return productService.getProduct(itemId);
    }

    @PostMapping(value = "/products-resync")
    public void resyncProducts(){

    }

//    @PostMapping(value = "/create-product")
//    public Mono<String> createProduct(@RequestBody Map<String, Object> request){
//        return productService.createProduct(request);
//    }

    @PostMapping(value = "/update-stock")
    public Mono<ResponseEntity<String>> updateStock(){
        stockService.updateStock("MSB").subscribe();

        return Mono.just(ResponseEntity.accepted().body("Update stock started"));
    }

    @PostMapping(value = "/sync-images")
    public Mono<Void> syncImages(){
        productService.syncImages().subscribe();

        return Mono.empty();
    }

    // User product
    @GetMapping(value = "/get-user-product")
    public Mono<String> getUserProduct(@RequestParam("item_id") String itemId){
        return productService.getUserProduct(itemId);
    }

    @PostMapping(value = "/update-shipping")
    public Mono<Void> updateShipping(){
        productService.updateShipping().subscribe();

        return Mono.empty();
    }

    @PostMapping(value = "/update-descriptions")
    public Mono<Void> updateDescription(){
        productService.updateDescription().subscribe();

        return Mono.empty();
    }
}
