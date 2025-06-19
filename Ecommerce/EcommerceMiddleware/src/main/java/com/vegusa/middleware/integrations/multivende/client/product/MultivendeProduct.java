package com.vegusa.middleware.integrations.multivende.client.product;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.Product;
import com.vegusa.middleware.integrations.multivende.dto.ProductAttributeDto;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;

@Component
public class MultivendeProduct {
    private final MultivendeClient multivendeClient;

    public MultivendeProduct(MultivendeClient multivendeClient) {
        this.multivendeClient = multivendeClient;
    }

    public Mono<EntriesDto<Product>> getAllProduct(){
        Map<String, String> queryParams = Map.of(
                "merchant_id", multivendeClient.getTokenStorage().getMerchantId(),
                "limit", "5000"
        );
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/m/{merchant_id}/products/limit/{limit}", queryParams)
                        .retrieve()
                        .bodyToMono(new ParameterizedTypeReference<EntriesDto<Product>>() {})
        );
    }

    public Mono<ProductAttributeDto> getAllAttributes(){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/m/{merchant_id}/all-product-attributes", merchantId)
                        .retrieve()
                        .bodyToMono(ProductAttributeDto.class)
        );
    }

    public Mono<Product> createProduct(Product product){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        ObjectMapper mapper = new ObjectMapper();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .post()
                        .uri("/api/m/{merchant_id}/products", merchantId)
                        .bodyValue(product)
                        .retrieve()
                        .bodyToMono(JsonNode.class)
                        .map(json -> {
                            JsonNode node = json.get("product");
                            try {
                                return mapper.treeToValue(node, Product.class);
                            } catch (Exception e){
                                throw new RuntimeException("Error");
                            }
                        })
        );
    }

    public Mono<JsonNode> updateProduct(Product product, String productId){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .put()
                        .uri("/api/products/{product_id}", productId)
                        .bodyValue(product)
                        .retrieve()
                        .bodyToMono(JsonNode.class)
                        .doOnSuccess(ignored -> {
                            System.out.println("id: " + productId);
                            System.out.println("Updated item: " + product.getInternalCode() + " - " + product.getCode());
                        })
                        .doOnError(error -> {
                            System.err.println(error.getMessage());
                        })
        );
    }

    public Mono<Product> getProductById(String productId){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/products/{product_id}", productId)
                        .retrieve()
                        .bodyToMono(Product.class)
        );
    }
}
