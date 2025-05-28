package com.vegusa.middleware.integrations.multivende.service;

import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;

import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;

@Service
public class OauthService {
    private final MultivendeClient multivendeClient;
    private final ScheduledExecutorService scheduler = Executors.newSingleThreadScheduledExecutor();

    @Autowired
    public OauthService(MultivendeClient multivendeClient){
        this.multivendeClient = multivendeClient;
    }

    @Scheduled(fixedRateString = "1800000", initialDelayString = "1800000")
    public void refreshTokenPeriodically() {
        multivendeClient.ensureValidAccessToken();
    }
}
