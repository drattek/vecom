package com.vegusa.middleware.oauth;

import com.vegusa.middleware.dto.IntegrationTokenRequest;
import com.vegusa.middleware.repository.local.IntegrationTokenRepository;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Autowired;

import java.time.Instant;

public abstract class TokenAbstract<T> {
    private volatile String accountId;
    private volatile String accessToken;
    private volatile String refreshToken;
    private volatile long expiresIn;
    private volatile long expiresRefreshIn;
    private volatile Instant expirationTime;
    private volatile Instant expirationRefreshTime;
    private volatile String apiKey;
    private volatile String apiSecret;
    private volatile Instant createdAt;
    private volatile Instant updatedAt;

    @Autowired
    private IntegrationTokenRepository integrationTokenRepository;

    public abstract String getIntegrationName();
    public abstract void save(IntegrationTokenRequest<T> request);

    @PostConstruct
    public void init() {
        integrationTokenRepository.findByIntegrationName(getIntegrationName()).ifPresent(token -> {
            this.accountId = token.getAccountId();
            this.accessToken = token.getAccessToken();
            this.refreshToken = token.getRefreshToken();
            this.expiresIn = token.getExpiresIn();
            this.expiresRefreshIn = token.getExpiresRefreshIn();
            this.expirationTime = token.getExpirationTime();
            this.expirationRefreshTime = token.getExpirationRefreshIn();
            this.apiKey = token.getApiKey();
            this.apiSecret = token.getApiSecret();
            this.createdAt = token.getCreatedAt();
            this.updatedAt = token.getUpdatedAt();

            System.out.println("Token data find in " + getIntegrationName());
        });
    }

    public String getAccountId() {
        return accountId;
    }

    public void setAccountId(String accountId) {
        this.accountId = accountId;
    }

    public String getAccessToken() {
        return accessToken;
    }

    public void setAccessToken(String accessToken) {
        this.accessToken = accessToken;
    }

    public String getRefreshToken() {
        return refreshToken;
    }

    public void setRefreshToken(String refreshToken) {
        this.refreshToken = refreshToken;
    }

    public long getExpiresIn() {
        return expiresIn;
    }

    public void setExpiresIn(long expiresIn) {
        this.expiresIn = expiresIn;
    }

    public long getExpiresRefreshIn() {
        return expiresRefreshIn;
    }

    public void setExpiresRefreshIn(long expiresRefreshIn) {
        this.expiresRefreshIn = expiresRefreshIn;
    }

    public String getApiKey() {
        return apiKey;
    }

    public void setApiKey(String apiKey) {
        this.apiKey = apiKey;
    }

    public String getApiSecret() {
        return apiSecret;
    }

    public void setApiSecret(String apiSecret) {
        this.apiSecret = apiSecret;
    }

    public Instant getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(Instant createdAt) {
        this.createdAt = createdAt;
    }

    public Instant getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(Instant updatedAt) {
        this.updatedAt = updatedAt;
    }

    public Instant getExpirationTime() {
        return expirationTime;
    }

    public void setExpirationTime(Instant expirationTime) {
        this.expirationTime = expirationTime;
    }

    public Instant getExpirationRefreshTime() {
        return expirationRefreshTime;
    }

    public void setExpirationRefreshTime(Instant expirationRefreshTime) {
        this.expirationRefreshTime = expirationRefreshTime;
    }

    public boolean isTokenExpired(){
        return this.expirationTime == null || Instant.now().isAfter(this.expirationTime);
    }

    public boolean isRefreshTokenExpired(){
        return this.expirationRefreshTime == null || Instant.now().isAfter(this.expirationRefreshTime);
    }
}
