package com.vegusa.middleware.integrations.multivende.service;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.time.Instant;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;

@Service
public class OauthService {
    private final MultivendeClient multivendeClient;
    private final ScheduledExecutorService scheduler = Executors.newSingleThreadScheduledExecutor();

    @Autowired
    public OauthService(MultivendeClient multivendeClient){
        this.multivendeClient = multivendeClient;
    }

    //@PostConstruct
    public void init() {
        Instant expiresAt = multivendeClient.getTokenStorage().getExpiresAt();
        Instant refreshExpiresAt = multivendeClient.getTokenStorage().getRefreshTokenExpiresAt();

        System.out.println("Now: " + Instant.now());
        long initialDelay = Duration.between(Instant.now(), expiresAt.minusSeconds(60)).toMillis(); // 1 min antes de expirar
        if (Instant.now().isAfter(refreshExpiresAt)){
            multivendeClient.performAuthentication().subscribe();
            initialDelay = Duration.ofHours(5).plusMinutes(55).toMillis();
        }
        initialDelay = Math.max(initialDelay, 0); // Evita delay negativo
        System.out.println("Initial delay: " + initialDelay);

        /*
        scheduler.scheduleAtFixedRate(
                this::refreshTokenPeriodically,
                initialDelay,
                Duration.ofHours(5).plusMinutes(55).toMillis(), //TimeUnit.HOURS.toMillis(5), // luego cada 5 horas y 55 minutos
                TimeUnit.MILLISECONDS
        );
        */
    }

    //@Scheduled(fixedRateString = "1800000", initialDelayString = "3000")
    public void refreshTokenPeriodically() {
        System.out.println("Start refreshing token");
        multivendeClient.refreshToken().subscribe();
    }
}
