package com.vegusa.middleware.integrations.mercadolibre.client.product;

import com.vegusa.middleware.integrations.mercadolibre.client.MercadolibreClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.product.ProductMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.oauth.TokenStorageMeli;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatusCode;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.Map;

@Component
public class ProductMeliClient {
    @Autowired
    private MercadolibreClient client;

    @Autowired
    private TokenStorageMeli tokenStorage;

    public Mono<String> getProducts(){
        return getProducts(null);
    }

    public Mono<String> getProducts(String healthy){
        return client.executeRateLimited(() ->
                client.getClient().get()
                        .uri(uriBuilder -> {
                            var builder = uriBuilder.path("/users/{user_id}/items/search");

                            if (healthy != null && !healthy.isBlank()){
                                builder = builder.queryParam("reputation_health_gauge", healthy);
                            }

                            return builder.build(tokenStorage.getAccountId());
                        }
                        )
                        .retrieve()
                        .bodyToMono(String.class)
        );
    }

    public Mono<ProductMeliDTO> getProduct(String itemId){
        return client.executeRateLimited(() ->
                client.getClient().get()
                        .uri("/items/{item_id}", itemId)
                        .retrieve()
                        .bodyToMono(ProductMeliDTO.class)
        );
    }

    public Mono<ProductMeliDTO> createProduct(ProductMeliDTO data){
        return client.executeRateLimited(() ->
                client.getClient().post()
                        .uri("/items")
                        .bodyValue(data)
                        .retrieve()
                        .onStatus(HttpStatusCode::is4xxClientError, clientResponse ->
                                clientResponse.bodyToMono(String.class)
                                        .doOnNext(errorBody -> System.err.println("Error 4xx: " + errorBody))
                                        .then(Mono.empty()) // No interrumpe el flujo
                        )
                        .bodyToMono(ProductMeliDTO.class)
                        .doOnError(error -> {
                            System.err.println(error.getMessage());
                        })
        );
    }

    public Mono<ProductMeliDTO> updateProduct(ProductMeliDTO product, String itemId){
        return client.executeRateLimited(() ->
                client.getClient().put()
                        .uri(uriBuilder -> uriBuilder
                                .path("/items/{item_id}")
                                .build(itemId)
                        )
                        .bodyValue(product)
                        .retrieve()
                        .onStatus(HttpStatusCode::is4xxClientError, clientResponse ->
                                clientResponse.bodyToMono(String.class)
                                        .flatMap(errorBody -> {
                                            HttpHeaders headers = clientResponse.headers().asHttpHeaders();
                                            String message = "Status: " + clientResponse.statusCode()
                                                    + ", Headers: " + headers
                                                    + ", Body: " + errorBody;
                                            return Mono.error(new RuntimeException(message));
                                        })
                        )
                        .bodyToMono(ProductMeliDTO.class)
        );
    }
}
