package com.vegusa.middleware.entity;

import com.vegusa.middleware.utils.EncryptDecryptConverter;
import com.vegusa.middleware.utils.JsonAttributeConverter;
import jakarta.persistence.*;

import java.time.Instant;
import java.util.Map;

@Entity
@Table(name = "integration_tokens")
public class IntegrationToken {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "id", nullable = false)
    private long id;

    @Column(name = "integration_name")
    private String integrationName;

    @Column(name = "account_id")
    private String accountId;

    @Column(name = "access_token")
    @Convert(converter = EncryptDecryptConverter.class)
    private String accessToken;

    @Column(name = "refresh_token")
    @Convert(converter = EncryptDecryptConverter.class)
    private String refreshToken;

    @Column(name = "scope")
    private String scope;

    @Column(name = "expires_in")
    private Long expiresIn;

    @Column(name = "expiration_time")
    private Instant expirationTime;

    @Column(name = "expires_refresh_in")
    private Long expiresRefreshIn;

    @Column(name = "expiration_refresh_time")
    private Instant expirationRefreshIn;

    @Column(name = "api_key")
    @Convert(converter = EncryptDecryptConverter.class)
    private String apiKey;

    @Column(name = "api_secret")
    @Convert(converter = EncryptDecryptConverter.class)
    private String apiSecret;

    @Convert(converter = JsonAttributeConverter.class)
    @Column(name = "extra_metadata")
    private Map<String, Object> extraMetadata;

    @Column(name = "created_at")
    private Instant createdAt;

    @Column(name = "updated_at")
    private Instant updatedAt;

    @Column(name = "integration_parameter_id")
    private Long integrationParameter;

    public long getId() {
        return id;
    }

    public void setId(long id) {
        this.id = id;
    }

    public String getIntegrationName() {
        return integrationName;
    }

    public void setIntegrationName(String integrationName) {
        this.integrationName = integrationName;
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

    public String getScope() {
        return scope;
    }

    public void setScope(String scope) {
        this.scope = scope;
    }

    public Long getExpiresIn() {
        return expiresIn;
    }

    public void setExpiresIn(Long expiresIn) {
        this.expiresIn = expiresIn;
    }

    public Instant getExpirationTime() {
        return expirationTime;
    }

    public void setExpirationTime(Instant expirationTime) {
        this.expirationTime = expirationTime;
    }

    public Long getExpiresRefreshIn() {
        return expiresRefreshIn;
    }

    public void setExpiresRefreshIn(Long expiresRefreshIn) {
        this.expiresRefreshIn = expiresRefreshIn;
    }

    public Instant getExpirationRefreshIn() {
        return expirationRefreshIn;
    }

    public void setExpirationRefreshIn(Instant expirationRefreshIn) {
        this.expirationRefreshIn = expirationRefreshIn;
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

    public Map<String, Object> getExtraMetadata() {
        return extraMetadata;
    }

    public void setExtraMetadata(Map<String, Object> extraMetadata) {
        this.extraMetadata = extraMetadata;
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

    public Long getIntegrationParameter() {
        return integrationParameter;
    }

    public void setIntegrationParameter(Long integrationParameter) {
        this.integrationParameter = integrationParameter;
    }
}
