package com.vegusa.middleware.integrations.multivende.client;

import com.vegusa.middleware.entity.AuthTokenParameter;
import com.vegusa.middleware.integrations.multivende.dto.OAuthDto;
import com.vegusa.middleware.integrations.multivende.dto.TokenRequest;
import com.vegusa.middleware.integrations.multivende.oauth.TokenStorage;
import com.vegusa.middleware.repository.AuthTokenParameterRepository;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.web.reactive.function.client.ClientRequest;
import org.springframework.web.reactive.function.client.ExchangeFilterFunction;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.core.publisher.Mono;
import reactor.core.publisher.Sinks;
import reactor.core.scheduler.Schedulers;

import java.time.Duration;
import java.util.Queue;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.function.Supplier;

@Component
public class MultivendeClient {
    private final TokenStorage tokenStorage;
    private final WebClient.Builder webClientBuilder;
    private WebClient webClient;

    @Autowired
    private AuthTokenParameterRepository authTokenParameterRepository;

    private final Queue<Supplier<Mono<?>>> requestQueue = new ConcurrentLinkedQueue<>();

    private volatile boolean isRefreshing = false;
    private volatile Mono<Void> refreshInProgress = Mono.empty();

    @Value("${multivende.api.base-url}")
    private String baseUrl;

    public MultivendeClient(WebClient.Builder webClientBuilder, TokenStorage tokenStorage) {
        this.webClientBuilder = webClientBuilder;
        this.tokenStorage = tokenStorage;
    }

    @PostConstruct
    public void init() {
        this.webClient = webClientBuilder
                .baseUrl(baseUrl)
                .defaultHeaders(headers -> {
                    headers.setContentType(MediaType.APPLICATION_JSON);
                })
                .filter(addAuthHeaderFilter())
                .codecs(codecs -> codecs.defaultCodecs().maxInMemorySize(50*1024*1024))
                .build();

        // Procesar una solicitud cada 250ms = 4 por segundo
        reactor.core.publisher.Flux.interval(Duration.ofMillis(250))
                .onBackpressureDrop()
                .publishOn(Schedulers.boundedElastic())
                .subscribe(tick -> {
                    Supplier<Mono<?>> task = requestQueue.poll();
                    if (task != null) {
                        Mono<?> taskMono = waitForRefresh().then(task.get());
                        taskMono.subscribe(); // Lanza la ejecución
                    }
                });
    }

    public WebClient getClient() {
        return this.webClient;
    }

    public <T> Mono<T> executeRateLimited(Supplier<Mono<T>> supplier) {
        Sinks.One<T> sink = Sinks.one();
        requestQueue.offer(() ->
                ensureValidAccessToken()
                        .then(waitForRefresh())
                        .then(supplier.get())
                        .doOnNext(sink::tryEmitValue)
                        .doOnError(sink::tryEmitError)
        );
        return sink.asMono();
    }

    public Mono<Void> ensureValidAccessToken() {
        if (!tokenStorage.isAccessTokenExpired()) {
            return Mono.empty(); // token aún es válido
        }

        if (tokenStorage.isRefreshTokenExpired()) {
            System.err.println("Ambos token han vencido. Se require una autenticación manual");
            return Mono.empty();
        }

        return refreshToken(); // Actualizar token
    }

    private Mono<Void> waitForRefresh() {
        if (!isRefreshing) return Mono.empty();
        return refreshInProgress;
    }

    public synchronized Mono<Void> refreshToken() {
        if (isRefreshing) {
            return refreshInProgress; // Ya se está haciendo un refresh
        }

        isRefreshing = true;
        Mono<Void> refresh = performRefreshToken()
                .doOnTerminate(() -> {
                    isRefreshing = false;
                    System.out.println("Access token refreshed");
                })
                .cache(); // Importante: compartir el mismo Mono con todos los que esperen

        refreshInProgress = refresh;
        return refresh;
    }

    private Mono<Void> performAuthentication() {
        AuthTokenParameter tokenParameter = authTokenParameterRepository.getTokenInfoParameters("MULTIVENDE");

        TokenRequest request = new TokenRequest();
        request.setClient_id(tokenParameter.getClientId());
        request.setClient_secret(tokenParameter.getClientSecret());
        request.setCode(tokenParameter.getAuthorizationCode());
        request.setGrant_type("authorization_code");

        return webClient.post()
                .uri("/oauth/access-token")
                .body(request, TokenRequest.class)
                .retrieve()
                .bodyToMono(OAuthDto.class)
                .doOnNext(tokenStorage::save)
                .then(); // Convertir a Mono<Void> para simplificar
    }

    private Mono<Void> performRefreshToken() {
        AuthTokenParameter tokenParameter = authTokenParameterRepository.getTokenInfoParameters("MULTIVENDE");

        TokenRequest request = new TokenRequest();
        request.setRefresh_token(tokenStorage.getRefreshToken());
        request.setClient_id(tokenParameter.getClientId());
        request.setClient_secret(tokenParameter.getClientSecret());
        request.setGrant_type("refresh_token");

        return webClient.post()
                .uri("/oauth/access-token")
                .bodyValue(request)
                .retrieve()
                .bodyToMono(OAuthDto.class)
                .doOnNext(tokenStorage::save)
                .then(); // Convertimos a Mono<Void> para simplificar
    }

    private ExchangeFilterFunction addAuthHeaderFilter() {
        return ExchangeFilterFunction.ofRequestProcessor(clientRequest -> {
            String token = tokenStorage.getAccessToken();
            System.out.println("token header: " + token);
            if (token != null && !token.isBlank()) {
                ClientRequest newRequest = ClientRequest.from(clientRequest)
                        .headers(headers -> headers.setBearerAuth(token))
                        .build();
                return Mono.just(newRequest);
            }
            return Mono.just(clientRequest);
        });
    }

    public TokenStorage getTokenStorage(){
        return tokenStorage;
    }
}
