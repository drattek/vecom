package com.vegusa.middleware.integrations.multivende.client.common;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.AppDto;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

@Component
public class MultivendeApp {

    private final MultivendeClient multivendeClient;

    public MultivendeApp(MultivendeClient multivendeClient){
        this.multivendeClient = multivendeClient;
    }

    public Mono<AppDto> getAppInfo(){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/d/info")
                        .retrieve()
                        .bodyToMono(AppDto.class)
        );
    }
}
