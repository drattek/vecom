package com.vegusa.middleware.integrations.jumpseller.client.product;

import com.fasterxml.jackson.databind.JsonNode;
import com.vegusa.middleware.integrations.jumpseller.client.JumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerProductDto;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

@Component
public class ProductJumpsellerClient {

    private final JumpsellerClient jumpsellerClient;

    public ProductJumpsellerClient(JumpsellerClient jumpsellerClient) {
        this.jumpsellerClient = jumpsellerClient;
    }

    public Mono<Integer> getProductCount() {
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .get()
                        .uri("/products/count.json")
                        .retrieve()
                        .bodyToMono(JsonNode.class)
                        .map(json -> json.get("count").asInt())
        );
    }

    public Mono<JumpsellerProductDto[]> getAllProducts(int page) {
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .get()
                        .uri(uriBuilder -> uriBuilder
                                .path("/products.json")
                                .queryParam("limit", 100)
                                .queryParam("page", page)
                                .build())
                        .retrieve()
                        .bodyToMono(JumpsellerProductDto[].class)
        );
    }

    public Mono<JumpsellerProductDto> getProductById(long id) {
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .get()
                        .uri("/products/{id}.json", id)
                        .retrieve()
                        .bodyToMono(JumpsellerProductDto.class)
        );
    }

    public Mono<JumpsellerProductDto> createProduct(JumpsellerProductDto product) {
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .post()
                        .uri("/products.json")
                        .bodyValue(product)
                        .retrieve()
                        .bodyToMono(JumpsellerProductDto.class)
        );
    }

    public Mono<JumpsellerProductDto> updateProduct(long id, JumpsellerProductDto product) {
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .put()
                        .uri("/products/{id}.json", id)
                        .bodyValue(product)
                        .retrieve()
                        .bodyToMono(JumpsellerProductDto.class)
                        .onErrorResume(e -> {
                            return Mono.empty();
                        })
        );
    }

    public Mono<Void> deleteProduct(String id) {
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .delete()
                        .uri("/products/{id}.json", id)
                        .retrieve()
                        .bodyToMono(Void.class)
        );
    }
}
