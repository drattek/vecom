package com.vegusa.middleware.integrations.jumpseller.client;

import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerProductDto;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.core.publisher.Mono;
import reactor.core.scheduler.Schedulers;
import reactor.core.publisher.Sinks;

import java.math.BigInteger;
import java.time.Duration;
import java.util.Queue;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.function.Supplier;

@Component
public class JumpsellerClient {

    private final WebClient.Builder webClientBuilder;
    private WebClient webClient;

    private final Queue<Supplier<Mono<?>>> requestQueue = new ConcurrentLinkedQueue<>();

    @Value("${jumpseller.api.base-url}")
    private String baseUrl;

    public JumpsellerClient(WebClient.Builder webClientBuilder) {
        this.webClientBuilder = webClientBuilder;
    }

    @PostConstruct
    public void init() {
        this.webClient = webClientBuilder
                .baseUrl(baseUrl)
                .defaultHeaders(headers -> {
                    headers.setBasicAuth("7f1efb9612750ba8c65be07f4a8f554e", "c6791d53c77d2fda2bc729ae4411690d");
                    headers.setContentType(MediaType.APPLICATION_JSON);
                })
                .build();

        // Procesar una solicitud cada 100ms = 10 por segundo
        reactor.core.publisher.Flux.interval(Duration.ofMillis(200))
                .onBackpressureDrop()
                .publishOn(Schedulers.boundedElastic())
                .subscribe(tick -> {
                    Supplier<Mono<?>> task = requestQueue.poll();
                    if (task != null) {
                        task.get().subscribe(); // Lanza la ejecución
                    }
                });
    }

    public WebClient getClient() {
        return this.webClient;
    }

    public <T> Mono<T> executeRateLimited(Supplier<Mono<T>> supplier) {
        Sinks.One<T> sink = Sinks.one();
        requestQueue.offer(() -> supplier.get().doOnNext(sink::tryEmitValue).doOnError(sink::tryEmitError));
        return sink.asMono();
    }
}
