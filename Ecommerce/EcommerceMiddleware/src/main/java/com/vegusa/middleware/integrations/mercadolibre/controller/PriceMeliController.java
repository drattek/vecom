package com.vegusa.middleware.integrations.mercadolibre.controller;

import com.vegusa.middleware.integrations.mercadolibre.service.price.PriceMeliService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

@RestController
@RequestMapping("/msb-ecommerce-middleware/mercadolibre")
public class PriceMeliController {
    @Autowired
    private PriceMeliService priceService;

    @GetMapping(value = "/get-prices")
    public Mono<String> getPrices(@RequestParam("item_id") String itemId){
        return priceService.getPrices(itemId);
    }

    @PostMapping(value = "/update-prices")
    public Mono<ResponseEntity<String>> updatePrices(){
        try {
            priceService.updatePrices("NORMAL", "MXN", "MSB").subscribe();
            return Mono.just(ResponseEntity.accepted().body("Update starting"));
        } catch (RuntimeException e){
            System.err.println("Error prices: " + e.getMessage());
        }
        return null;
    }
}
