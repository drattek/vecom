package com.vegusa.middleware.integrations.multivende.client.product;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import org.springframework.stereotype.Component;

@Component
public class MultivendeProduct {

    private final MultivendeClient multivendeClient;

    public MultivendeProduct(MultivendeClient multivendeClient) {
        this.multivendeClient = multivendeClient;
    }
}
