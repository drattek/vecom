package com.vegusa.middleware.integrations.multivende.client.product;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.Tag;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;

@Component
public class MultivendeTag {
    private final MultivendeClient multivendeClient;

    public MultivendeTag(MultivendeClient multivendeClient) {
        this.multivendeClient = multivendeClient;
    }

    public Mono<List<Tag>> getAllTag(){
        Map<String, String> queryParams = Map.of(
                "merchant_id", multivendeClient.getTokenStorage().getMerchantId(),
                "page", "1"
        );
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/m/{{merchant_id}}/tags/p/{page}", queryParams)
                        .retrieve()
                        .bodyToMono(new ParameterizedTypeReference<EntriesDto<Tag>>() {})
                        .map(EntriesDto::getEntries)
        );
    }

    public Mono<Tag> createTag(Tag tag){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .post()
                        .uri("/api/m/{{merchant_id}}/tags/type/_product_tag", merchantId)
                        .bodyValue(tag)
                        .retrieve()
                        .bodyToMono(Tag.class)
        );
    }

    public Mono<Tag> updateTag(Tag tag, String tagId){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .put()
                        .uri("/api/tags/{{tag_id}}", tagId)
                        .bodyValue(tag)
                        .retrieve()
                        .bodyToMono(Tag.class)
        );
    }

    public Mono<Void> deleteTag(String tagId){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .delete()
                        .uri("/api/tags/{{tag_id}}", tagId)
                        .retrieve()
                        .bodyToMono(Void.class)
        );
    }
}
