package com.vegusa.middleware.integrations.multivende.service.product;

import com.vegusa.middleware.integrations.multivende.client.product.MultivendeProduct;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.Product;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

@Service
public class ProductService {
    private final MultivendeProduct multivendeProduct;

    @Autowired
    public ProductService(MultivendeProduct multivendeProduct) {
        this.multivendeProduct = multivendeProduct;
    }

    public Mono<EntriesDto<Product>> getAllProduct(){
        return multivendeProduct.getAllProduct();
    }
}
