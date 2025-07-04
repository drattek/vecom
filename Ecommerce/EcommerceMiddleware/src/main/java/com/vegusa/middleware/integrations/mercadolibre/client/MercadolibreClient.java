package com.vegusa.middleware.integrations.mercadolibre.client;

import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.constants.TokenType;
import com.vegusa.middleware.dto.IntegrationTokenRequest;
import com.vegusa.middleware.entity.IntegrationParameter;
import com.vegusa.middleware.integrations.mercadolibre.dto.OauthMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.oauth.TokenStorageMeli;
import com.vegusa.middleware.repository.IntegrationParameterRepository;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatusCode;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.util.LinkedMultiValueMap;
import org.springframework.util.MultiValueMap;
import org.springframework.web.reactive.function.BodyInserters;
import org.springframework.web.reactive.function.client.ClientRequest;
import org.springframework.web.reactive.function.client.ExchangeFilterFunction;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.reactive.function.client.WebClientResponseException;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;
import reactor.core.publisher.Sinks;
import reactor.core.scheduler.Schedulers;
import reactor.util.retry.Retry;

import java.time.Duration;
import java.util.Queue;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.function.Supplier;

@Component
public class MercadolibreClient {
    private final WebClient.Builder webClientBuilder;
    private WebClient webClient;

    private final Queue<Supplier<Mono<?>>> requestQueue = new ConcurrentLinkedQueue<>();

    private volatile boolean isRefreshing = false;
    private volatile Mono<Void> refreshInProgress = Mono.empty();

    @Value("${mercadolibre.api.base-url}")
    private String baseUrl;

    @Autowired
    private TokenStorageMeli tokenStorage;

    @Autowired
    private IntegrationParameterRepository integrationParameterRepository;

    public MercadolibreClient(WebClient.Builder webClientBuilder) {
        this.webClientBuilder = webClientBuilder;
    }

    @PostConstruct
    public void init(){
        this.webClient = webClientBuilder
                .baseUrl(baseUrl)
                .defaultHeaders(headers -> {
                    headers.setContentType(MediaType.APPLICATION_JSON);
                })
                .filter(addAuthHeaderFilter())
                .build();

        Flux.interval(Duration.ofSeconds(2))
                .onBackpressureBuffer()
                .publishOn(Schedulers.boundedElastic())
                .subscribe(tick -> {
                    Supplier<Mono<?>> task = requestQueue.poll();
                    if (task != null){
                        Mono<?> taskMono = waitForRefresh().then(task.get());
                        taskMono.subscribe();
                    }
                });
    }

    public WebClient getClient(){
        return this.webClient;
    }

    public <T> Mono<T> executeRateLimited(Supplier<Mono<T>> supplier){
        Sinks.One<T> sink = Sinks.one();
        requestQueue.offer(() ->
            ensureValidAccessToken()
                    .then(waitForRefresh())
                    .then(supplier.get()
                            .retryWhen(Retry.backoff(3, Duration.ofSeconds(2))
                                    .filter(this::isRetrayable)
                                    .onRetryExhaustedThrow((retryBackoffSpec, retrySignal) -> retrySignal.failure())
                            )
                    )
                    .doOnNext(sink::tryEmitValue)
                    .doOnError(sink::tryEmitError)
        );

        return sink.asMono();
    }

    private Mono<Void> waitForRefresh() {
        if (!isRefreshing) return Mono.empty();
        return refreshInProgress;
    }

    public synchronized Mono<Void> refreshToken(){
        if (isRefreshing){
            return refreshInProgress;
        }

        isRefreshing = true;
        Mono<Void> refresh = performRefreshToken()
                .doOnTerminate(() -> {
                    isRefreshing = false;
                })
                .cache();

        refreshInProgress = refresh;
        return refresh;
    }

    public Mono<Void> performRefreshToken() {
        WebClient formClient = webClientBuilder
                .baseUrl(baseUrl)
                .build();

        MultiValueMap<String, String> formData = new LinkedMultiValueMap<>();
        formData.add("grant_type", "refresh_token");
        formData.add("client_id", tokenStorage.getClientId());
        formData.add("client_secret", tokenStorage.getClientSecret());
        formData.add("refresh_token", tokenStorage.getRefreshToken());

        System.out.println("Token: " + tokenStorage.getRefreshToken());
        System.out.println("Client: " + tokenStorage.getClientId());
        System.out.println("Secret: " + tokenStorage.getClientSecret());

        return formClient.post()
                .uri("/oauth/token")
                .contentType(MediaType.APPLICATION_FORM_URLENCODED)
                .body(BodyInserters.fromFormData(formData))
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
                .bodyToMono(OauthMeliDTO.class)
                .doOnNext(response -> {
                    System.out.println("Mercado libre token refresh successfully");
                    IntegrationTokenRequest<OauthMeliDTO> token = new IntegrationTokenRequest<>();
                    token.setIntegrationName(tokenStorage.getIntegrationName());
                    token.setTokenType(TokenType.BEARER);
                    token.setData(response);

                    tokenStorage.save(token);
                })
                .then();
    }

    public Mono<Void> performAccessToken(String code){
        WebClient formClient = webClientBuilder
                .baseUrl(baseUrl)
                .defaultHeader("Content-Type", MediaType.APPLICATION_FORM_URLENCODED_VALUE)
                .build();
        String code_verifier = integrationParameterRepository.getVerifier(IntegrationType.MERCADO_LIBRE.name());

        MultiValueMap<String, String> formData = new LinkedMultiValueMap<>();
        formData.add("grant_type", "authorization_code");
        formData.add("client_id", tokenStorage.getClientId());
        formData.add("client_secret", tokenStorage.getClientSecret());
        formData.add("code", code);
        formData.add("redirect_uri", tokenStorage.getStoreUrl());
        formData.add("code_verifier", code_verifier);

        return formClient.post()
                .uri("/oauth/token")
                .contentType(MediaType.APPLICATION_FORM_URLENCODED)
                .body(BodyInserters.fromFormData(formData))
                .retrieve()
                .bodyToMono(OauthMeliDTO.class)
                .doOnNext(response -> {
                    IntegrationTokenRequest<OauthMeliDTO> token = new IntegrationTokenRequest<>();
                    token.setIntegrationName(tokenStorage.getIntegrationName());
                    token.setTokenType(TokenType.BEARER);
                    token.setData(response);

                    tokenStorage.save(token);
                })
                .doOnError(e -> {
                    System.err.println("Error: " + e.getMessage());
                })
                .then();
    }

    private ExchangeFilterFunction addAuthHeaderFilter(){
        return ExchangeFilterFunction.ofRequestProcessor(clientRequest -> {
            String token = tokenStorage.getAccessToken();
            if (token != null && !token.isBlank()){
                ClientRequest newRequest = ClientRequest.from(clientRequest)
                        .headers(httpHeaders -> httpHeaders.setBearerAuth(token))
                        .build();
                return Mono.just(newRequest);
            }
            return Mono.just(clientRequest);
        });
    }

    private Mono<Void> ensureValidAccessToken(){
        if (!tokenStorage.isTokenExpired()){
            return Mono.empty();
        }

        if (tokenStorage.isRefreshTokenExpired()){
            return Mono.empty();
        }

        return refreshToken();
    }

    private boolean isRetrayable(Throwable throwable){
        if (throwable instanceof WebClientResponseException e){
            int status = e.getStatusCode().value();
            return (status >= 500 && status < 600) || (status >= 400 && status < 500 && status != 401 && status != 403);
        }
        return false;
    }
}
