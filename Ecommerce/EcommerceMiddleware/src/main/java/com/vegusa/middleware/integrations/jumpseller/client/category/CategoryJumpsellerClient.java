package com.vegusa.middleware.integrations.jumpseller.client.category;

import com.fasterxml.jackson.databind.JsonNode;
import com.vegusa.middleware.integrations.jumpseller.client.JumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerCategoryDto;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

@Component
public class CategoryJumpsellerClient {

    private final JumpsellerClient jumpsellerClient;

    public CategoryJumpsellerClient(JumpsellerClient jumpsellerClient){
        this.jumpsellerClient = jumpsellerClient;
    }

    public Mono<Integer> getCategoryCount(){
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .get()
                        .uri("/categories/count.json")
                        .retrieve()
                        .bodyToMono(JsonNode.class)
                        .map(json -> json.get("count").asInt())
        );
    }

    public Mono<JumpsellerCategoryDto[]> getAllCategories() {
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .get()
                        .uri("/categories.json")
                        .retrieve()
                        .bodyToMono(JumpsellerCategoryDto[].class)
        );
    }

    public Mono<JumpsellerCategoryDto> getCategoryById(long id){
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .get()
                        .uri("/categories/{id}.json", id)
                        .retrieve()
                        .bodyToMono(JumpsellerCategoryDto.class)
        );
    }

    public Mono<JumpsellerCategoryDto> createCategory(JumpsellerCategoryDto category){
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .post()
                        .uri("/categories.json")
                        .bodyValue(category)
                        .retrieve()
                        .bodyToMono(JumpsellerCategoryDto.class)
        );
    }

    public Mono<JumpsellerCategoryDto> updateCategory(long id, JumpsellerCategoryDto category){
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .put()
                        .uri("/categories/{id}.json", id)
                        .bodyValue(category)
                        .retrieve()
                        .bodyToMono(JumpsellerCategoryDto.class)
        );
    }

    public Mono<Void> deleteCategory(long id){
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .delete()
                        .uri("/categories/{id}.json")
                        .retrieve()
                        .bodyToMono(Void.class)
        );
    }
}
