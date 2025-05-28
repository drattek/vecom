package com.vegusa.middleware.integrations.multivende.controller.product;

import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.Product;
import com.vegusa.middleware.integrations.multivende.service.product.ProductService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

@RestController
@RequestMapping("msb-ecommerce-middleware/multivende")
public class MultivendeProductController {
    @Autowired
    private ProductService productService;

    @GetMapping(value = "/get-products")
    public Mono<EntriesDto<Product>> getAllProduct(){
        return productService.getAllProduct();
    }
}
