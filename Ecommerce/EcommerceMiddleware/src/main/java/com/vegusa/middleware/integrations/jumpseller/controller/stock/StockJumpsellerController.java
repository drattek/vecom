package com.vegusa.middleware.integrations.jumpseller.controller.stock;

import com.vegusa.middleware.integrations.jumpseller.dto.StockDto;
import com.vegusa.middleware.integrations.jumpseller.service.stock.StockJumpsellerService;
import com.vegusa.middleware.utils.MWUtils;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

import java.util.HashMap;
import java.util.List;

@RestController
@RequestMapping("msb-ecommerce-middleware/jumpseller")
public class StockJumpsellerController {
    @Autowired
    private StockJumpsellerService stockJumpsellerService;

    @Autowired
    public StockJumpsellerController(){}

    @GetMapping(value = "/get-stock")
    public Mono<List<StockDto>> getStock(){
        try {
            return stockJumpsellerService.getStock();
        } catch (RuntimeException e){
            System.err.println("Error: " + e.getMessage());
        }
        return null;
    }

    @PostMapping(value = "/update-stock")
    public Mono<ResponseEntity<String>> updateStock(@RequestBody HashMap<String, String> request){
        String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId"));
        stockJumpsellerService.updateStock(dataAreaId).subscribe();

        return Mono.just(ResponseEntity.accepted().body("Update started"));
    }
}
