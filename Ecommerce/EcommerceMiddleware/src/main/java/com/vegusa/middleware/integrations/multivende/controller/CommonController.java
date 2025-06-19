package com.vegusa.middleware.integrations.multivende.controller;

import com.vegusa.middleware.integrations.multivende.client.pricelist.MultivendePricelist;
import com.vegusa.middleware.integrations.multivende.dto.Pricelist;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

import java.util.List;

@RestController
@RequestMapping("msb-ecommerce-middleware/multivende")
public class CommonController {
    @Autowired
    private MultivendePricelist multivendePricelist;

    @GetMapping(value = "/get-pricelist")
    public Mono<List<Pricelist>> getPricelist(){
        return multivendePricelist.getAllPricelist();
    }
}
