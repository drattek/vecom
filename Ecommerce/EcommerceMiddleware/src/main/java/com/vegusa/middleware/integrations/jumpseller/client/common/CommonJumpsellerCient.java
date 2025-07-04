package com.vegusa.middleware.integrations.jumpseller.client.common;

import com.vegusa.middleware.integrations.jumpseller.client.JumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerInfoDto;
import com.vegusa.middleware.integrations.jumpseller.dto.LanguageDto;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

@Component
public class CommonJumpsellerCient {

    private final JumpsellerClient jumpsellerClient;

    public CommonJumpsellerCient(JumpsellerClient jumpsellerClient) {
        this.jumpsellerClient = jumpsellerClient;
    }

    public Mono<JumpsellerInfoDto> getAppInfo(){
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .get()
                        .uri("/store/info.json")
                        .retrieve()
                        .bodyToMono(JumpsellerInfoDto.class)
        );
    }

    public Mono<LanguageDto> getLanguages(){
        return jumpsellerClient.executeRateLimited(() ->
                jumpsellerClient.getClient()
                        .get()
                        .uri("/store/languages.json")
                        .retrieve()
                        .bodyToMono(LanguageDto.class)
        );
    }
}
