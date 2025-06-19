package com.vegusa.middleware.integrations.multivende.controller.product;

import com.vegusa.middleware.dto.Request;
import com.vegusa.middleware.entity.Company;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.Product;
import com.vegusa.middleware.integrations.multivende.service.product.ProductService;
import com.vegusa.middleware.utils.CommonUtils;
import jakarta.validation.Valid;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

import java.util.HashMap;

@RestController
@RequestMapping("msb-ecommerce-middleware/multivende")
public class MultivendeProductController {
    @Autowired
    private ProductService productService;

    @Autowired
    private CommonUtils commonUtils;

    @GetMapping(value = "/get-products")
    public Mono<EntriesDto<Product>> getAllProduct(){
        return productService.getAllProduct();
    }

    @PostMapping(value = "/create-product")
    public Mono<ResponseEntity<String>> createProduct(){

        return Mono.just(ResponseEntity.accepted().body("Product creation"));
    }

    @PostMapping(value = "/update-products")
    private Mono<ResponseEntity<String>> updateProducts(@RequestBody @Valid Request request){
        Company company = commonUtils.getCompany(request.getDataAreaId());
        productService.updateProducts(company).subscribe();

        return Mono.just(ResponseEntity.accepted().body("Updating products"));
    }
}
