package com.vegusa.middleware.integrations.camso.client;

import com.vegusa.middleware.constants.TokenType;
import com.vegusa.middleware.dto.IntegrationTokenRequest;
import com.vegusa.middleware.integrations.camso.dto.OauthCamsoDTO;
import com.vegusa.middleware.integrations.camso.oauth.TokenStorageCamso;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.util.LinkedMultiValueMap;
import org.springframework.util.MultiValueMap;
import org.springframework.web.reactive.function.BodyInserters;
import org.springframework.web.reactive.function.client.ClientRequest;
import org.springframework.web.reactive.function.client.ExchangeFilterFunction;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.core.publisher.Mono;

import java.util.Queue;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.function.Supplier;

@Component
public class CamsoClient {
    private final WebClient.Builder webClientBuilder;
    private WebClient webClient;

    private final Queue<Supplier<Mono<?>>> requestQueue = new ConcurrentLinkedQueue<>();

    private volatile boolean isRefreshing = false;
    private volatile Mono<Void> refreshingInProgress = Mono.empty();

    @Value("${camso.api.base-url}")
    private String baseUrl;

    @Autowired
    private TokenStorageCamso tokenStorage;

    public CamsoClient(WebClient.Builder webClientBuilder) {
        this.webClientBuilder = webClientBuilder;
    }

    @PostConstruct
    public void init(){
        this.webClient = webClientBuilder
                .baseUrl(baseUrl)
                .filter(addAuthHeaderFilter())
                .build();
    }

    public WebClient getClient(){
        return this.webClient;
    }

    public Mono<Void> performAccessToken(){
        WebClient formClient = webClientBuilder
                .baseUrl(baseUrl)
                .defaultHeaders(httpHeaders -> {
                    httpHeaders.setContentType(MediaType.APPLICATION_FORM_URLENCODED);
                    httpHeaders.setBasicAuth(tokenStorage.getClientSecret());
                })
                .build();

        MultiValueMap<String, String> formData = new LinkedMultiValueMap<>();
        formData.add("grant_type", "client_credentials");

        return formClient.post()
                .uri("/idp/v2/b2b/oauth2/token")
                .contentType(MediaType.APPLICATION_FORM_URLENCODED)
                .body(BodyInserters.fromFormData(formData))
                .retrieve()
                .bodyToMono(OauthCamsoDTO.class)
                .doOnNext(response -> {
                    System.out.println("Authentification successfully");
                    IntegrationTokenRequest<OauthCamsoDTO> token = new IntegrationTokenRequest<>();
                    token.setIntegrationName(tokenStorage.getIntegrationName());
                    token.setTokenType(TokenType.BASIC_AUTH);
                    token.setData(response);

                    tokenStorage.save(token);
                })
                .doOnError(error -> System.err.println("Error authentication: " + error.getMessage()))
                .then();
    }

    private ExchangeFilterFunction addAuthHeaderFilter(){
        return ExchangeFilterFunction.ofRequestProcessor(clientRequest -> {
            String token = tokenStorage.getAccessToken();
            String apiKey = tokenStorage.getApiKey();

            System.out.println("Apikey: " + apiKey);
            System.out.println("Token: " + token);
            if (!token.isEmpty() && !apiKey.isEmpty()){
                ClientRequest newRequest = ClientRequest.from(clientRequest)
                        .headers(httpHeaders -> {
                            httpHeaders.set("token", token);
                            httpHeaders.set("apiKey", apiKey);
                        })
                        .build();

                return Mono.just(newRequest);
            }
            return Mono.just(clientRequest);
        });
    }
}
