package com.vegusa.middleware.integrations.mercadolibre.client.common;

import com.vegusa.middleware.integrations.mercadolibre.client.MercadolibreClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.common.CurrencyMeliDTO;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

@Component
public class CommonMeliClient {
    @Autowired
    private MercadolibreClient client;

    public Mono<CurrencyMeliDTO[]> getCurrencies(){
        return client.executeRateLimited(() ->
                client.getClient().get()
                        .uri("/currencies")
                        .retrieve()
                        .bodyToMono(CurrencyMeliDTO[].class)
        );
    }
}
