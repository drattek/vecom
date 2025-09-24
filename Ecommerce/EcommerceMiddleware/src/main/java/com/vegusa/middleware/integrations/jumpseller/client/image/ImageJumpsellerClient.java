package com.vegusa.middleware.integrations.jumpseller.client.image;

import com.vegusa.middleware.integrations.jumpseller.client.JumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerImageDTO;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

@Component
public class ImageJumpsellerClient {
    @Autowired
    private JumpsellerClient client;

    public ImageJumpsellerClient() {}

    public Mono<JumpsellerImageDTO[]> getProductImages(String productId){
        return client.executeRateLimited(() ->
                client.getClient().get()
                        .uri("/products/{id}/images.json", productId)
                        .retrieve()
                        .bodyToMono(JumpsellerImageDTO[].class)
        );
    }

    public Mono<JumpsellerImageDTO> uploadImage(String productId, JumpsellerImageDTO image){
        return client.executeRateLimited(() ->
                client.getClient().post()
                        .uri("/products/{id}/images.json", productId)
                        .bodyValue(image)
                        .retrieve()
                        .bodyToMono(JumpsellerImageDTO.class)
        );
    }
}
