package com.vegusa.middleware.integrations.multivende.client.product;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.Category;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;

@Component
public class MultivendeCategory {
    private final MultivendeClient multivendeClient;

    public MultivendeCategory(MultivendeClient multivendeClient) {
        this.multivendeClient = multivendeClient;
    }

    public Mono<List<Category>> getAllCategory(){
        Map<String, String> queryParams = Map.of(
                "merchant_id", multivendeClient.getTokenStorage().getMerchantId(),
                "page", "1"
        );
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/m/{{merchant_id}}/product-categories/p/{{page}}", queryParams)
                        .retrieve()
                        .bodyToMono(new ParameterizedTypeReference<EntriesDto<Category>>() {})
                        .map(EntriesDto::getEntries)
        );
    }

    public Mono<Category> createCategory(Category category){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .post()
                        .uri("/api/m/{{merchant_id}}/product-categories", merchantId)
                        .bodyValue(category)
                        .retrieve()
                        .bodyToMono(Category.class)
        );
    }

    public Mono<Category> updateCategory(Category category, String id){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .put()
                        .uri("/api/product-categories/{{product_category_id}}", id)
                        .bodyValue(category)
                        .retrieve()
                        .bodyToMono(Category.class)
        );
    }

    public Mono<Category> getCategoryById(String id){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/product-categories/{{product_category_id}}", id)
                        .retrieve()
                        .bodyToMono(Category.class)
        );
    }
}
