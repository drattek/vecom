package com.vegusa.middleware.integrations.multivende.client;

import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
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
    private final WebClient.Builder webClientBuilder;
    private WebClient webClient;

    private final Queue<Supplier<Mono<?>>> requestQueue = new ConcurrentLinkedQueue<>();

    @Value("${multivende.api.base-url}")
    private String baseUrl;

    public MultivendeClient(WebClient.Builder webClientBuilder) {
        this.webClientBuilder = webClientBuilder;
    }

    @PostConstruct
    public void init() {
        this.webClient = webClientBuilder
                .baseUrl(baseUrl)
                .defaultHeaders(headers -> {
                    headers.setContentType(MediaType.APPLICATION_JSON);
                })
                .build();

        // Procesar una solicitud cada 250ms = 4 por segundo
        reactor.core.publisher.Flux.interval(Duration.ofMillis(250))
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
