package com.vegusa.middleware.integrations.multivende.client.product;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.Season;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;

@Component
public class MultivendeSeason {
    private final MultivendeClient multivendeClient;

    public MultivendeSeason(MultivendeClient multivendeClient) {
        this.multivendeClient = multivendeClient;
    }

    public Mono<List<Season>> getAllSeason(){
        Map<String, String> queryParams = Map.of(
                "merchant_id", multivendeClient.getTokenStorage().getMerchantId(),
                "page", "1"
        );
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/m/{{merchant_id}}/seasons/p/{{page}}", queryParams)
                        .retrieve()
                        .bodyToMono(new ParameterizedTypeReference<EntriesDto<Season>>() {})
                        .map(EntriesDto::getEntries)
        );
    }

    public Mono<Season> createSeason(Season season){
        String merchantId = multivendeClient.getTokenStorage().getMerchantId();
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .post()
                        .uri("/api/m/{{merchant_id}}/seasons", merchantId)
                        .bodyValue(season)
                        .retrieve()
                        .bodyToMono(Season.class)
        );
    }

    public Mono<Season> updateSeason(Season season, String id){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .put()
                        .uri("/api/seasons/{{season_id}}", id)
                        .bodyValue(season)
                        .retrieve()
                        .bodyToMono(Season.class)
        );
    }

    public Mono<Season> getSeasonById(String id){
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .get()
                        .uri("/api/seasons/{{season_id}}", id)
                        .retrieve()
                        .bodyToMono(Season.class)
        );
    }
}
