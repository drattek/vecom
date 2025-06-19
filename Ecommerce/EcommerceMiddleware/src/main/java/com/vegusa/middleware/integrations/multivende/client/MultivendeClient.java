package com.vegusa.middleware.integrations.multivende.client;

import com.vegusa.middleware.entity.AuthTokenParameter;
import com.vegusa.middleware.integrations.multivende.dto.OAuthDto;
import com.vegusa.middleware.integrations.multivende.dto.TokenRequest;
import com.vegusa.middleware.integrations.multivende.oauth.TokenStorageMultivende;
import com.vegusa.middleware.repository.AuthTokenParameterRepository;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.web.reactive.function.client.ClientRequest;
import org.springframework.web.reactive.function.client.ExchangeFilterFunction;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.reactive.function.client.WebClientResponseException;
import reactor.core.publisher.Mono;
import reactor.core.publisher.Sinks;
import reactor.core.scheduler.Schedulers;
import reactor.util.retry.Retry;

import java.time.Duration;
import java.util.Queue;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.function.Supplier;

@Component
public class MultivendeClient {
    private final TokenStorageMultivende tokenStorageMultivende;
    private final WebClient.Builder webClientBuilder;
    private WebClient webClient;

    @Autowired
    private AuthTokenParameterRepository authTokenParameterRepository;

    private final Queue<Supplier<Mono<?>>> requestQueue = new ConcurrentLinkedQueue<>();

    private volatile boolean isRefreshing = false;
    private volatile Mono<Void> refreshInProgress = Mono.empty();

    @Value("${multivende.api.base-url}")
    private String baseUrl;

    public MultivendeClient(WebClient.Builder webClientBuilder, TokenStorageMultivende tokenStorageMultivende) {
        this.webClientBuilder = webClientBuilder;
        this.tokenStorageMultivende = tokenStorageMultivende;
    }

    @PostConstruct
    public void init() {
        /*
        HttpClient httpClient = HttpClient.create()
                .wiretap("reactor.netty.http.client.HttpClient",
                        LogLevel.DEBUG,
                        AdvancedByteBufFormat.TEXTUAL);

         */

        this.webClient = webClientBuilder
                //.clientConnector(new ReactorClientHttpConnector(httpClient))
                .baseUrl(baseUrl)
                .defaultHeaders(headers -> {
                    headers.setContentType(MediaType.APPLICATION_JSON);
                })
                .filter(addAuthHeaderFilter())
                .codecs(codecs -> codecs.defaultCodecs().maxInMemorySize(50*1024*1024))
                .build();

        // Procesar una solicitud cada 250ms = 4 por segundo
        reactor.core.publisher.Flux.interval(Duration.ofMillis(250))
                .onBackpressureBuffer()
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
                        .then(supplier.get()
                                .retryWhen(Retry.backoff(3, Duration.ofSeconds(2))
                                        .filter(this::isRetryable)
                                        .onRetryExhaustedThrow((retryBackoffSpec, retrySignal) -> retrySignal.failure())
                                )
                        )
                        .doOnNext(sink::tryEmitValue)
                        .doOnError(sink::tryEmitError)
        );
        return sink.asMono();
    }

    private boolean isRetryable(Throwable throwable) {
        if (throwable instanceof WebClientResponseException e) {
            int status = e.getStatusCode().value();
            return (status >= 500 && status < 600) || (status >= 400 && status < 500 && status != 401 && status != 403);
        }
        return false;
    }

    public Mono<Void> ensureValidAccessToken() {
        if (!tokenStorageMultivende.isAccessTokenExpired()) {
            return Mono.empty(); // token aún es válido
        }

        if (tokenStorageMultivende.isRefreshTokenExpired()) {
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
                })
                .cache(); // Importante: compartir el mismo Mono con todos los que esperen

        refreshInProgress = refresh;
        return refresh;
    }

    public Mono<Void> performAuthentication() {
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
                .doOnNext(tokenStorageMultivende::save)
                .then(); // Convertir a Mono<Void> para simplificar
    }

    private Mono<Void> performRefreshToken() {
        AuthTokenParameter tokenParameter = authTokenParameterRepository.getTokenInfoParameters("MULTIVENDE");

        TokenRequest request = new TokenRequest();
        request.setRefresh_token(tokenStorageMultivende.getRefreshToken());
        request.setClient_id(tokenParameter.getClientId());
        request.setClient_secret(tokenParameter.getClientSecret());
        request.setGrant_type("refresh_token");

        System.out.println("Refresh token: " + tokenStorageMultivende.getRefreshToken());
        return webClient.post()
                .uri("/oauth/access-token")
                .bodyValue(request)
                .retrieve()
                .bodyToMono(OAuthDto.class)
                .doOnNext(tokenStorageMultivende::save)
                .then(); // Convertimos a Mono<Void> para simplificar
    }

    private ExchangeFilterFunction addAuthHeaderFilter() {
        return ExchangeFilterFunction.ofRequestProcessor(clientRequest -> {
            String token = tokenStorageMultivende.getAccessToken();
            if (token != null && !token.isBlank()) {
                ClientRequest newRequest = ClientRequest.from(clientRequest)
                        .headers(headers -> headers.setBearerAuth(token))
                        .build();
                return Mono.just(newRequest);
            }
            return Mono.just(clientRequest);
        });
    }

    public TokenStorageMultivende getTokenStorage(){
        return tokenStorageMultivende;
    }
}
