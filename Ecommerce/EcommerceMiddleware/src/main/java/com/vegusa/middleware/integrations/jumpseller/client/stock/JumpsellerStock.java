package com.vegusa.middleware.integrations.jumpseller.client.stock;

import com.vegusa.middleware.integrations.jumpseller.client.JumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.dto.StockDto;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.List;

@Component
public class JumpsellerStock {
    private final JumpsellerClient jumpsellerClient;

    public  JumpsellerStock(JumpsellerClient jumpsellerClient){
        this.jumpsellerClient = jumpsellerClient;
    }

    public Mono<StockDto[]> getStock(List<String> products){
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .get()
                        .uri(uriBuilder -> uriBuilder
                                .path("/products_locations.json")
                                .queryParam("location_ids", List.of("238677"))
                                .queryParam("product_ids", products)
                                .build())
                        .retrieve()
                        .bodyToMono(StockDto[].class)
        );
    }
}
