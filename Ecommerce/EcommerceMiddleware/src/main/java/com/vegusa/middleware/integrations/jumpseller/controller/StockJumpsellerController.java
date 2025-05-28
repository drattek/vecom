package com.vegusa.middleware.integrations.jumpseller.controller;

import com.vegusa.middleware.integrations.jumpseller.dto.StockDto;
import com.vegusa.middleware.integrations.jumpseller.service.JumpsellerStockService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

import java.util.List;

@RestController
@RequestMapping("msb-ecommerce-middleware/jumpseller")
public class StockJumpsellerController {
    @Autowired
    private JumpsellerStockService jumpsellerStockService;

    @Autowired
    public StockJumpsellerController(){}

    @GetMapping(value = "/get-stock")
    public Mono<List<StockDto>> getStock(){
        try {
            return jumpsellerStockService.getStock();
        } catch (RuntimeException e){
            System.err.println("Error: " + e.getMessage());
        }
        return null;
    }
}
