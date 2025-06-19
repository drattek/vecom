package com.vegusa.middleware.integrations.multivende.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.time.Instant;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class OAuthDto {
    @JsonProperty("_id")
    private String _id;

    @JsonProperty("status")
    private String status;

    @JsonProperty("OauthClientId")
    private String oauthClientId;

    @JsonProperty("MerchantId")
    private String merchantId;

    @JsonProperty("CreatedById")
    private String createdById;

    @JsonProperty("UpdatedById")
    private String updatedById;

    @JsonProperty("OwnerId")
    private String ownerId;

    @JsonProperty("expiresAt")
    private Instant expiresAt;

    @JsonProperty("refreshToken")
    private String refreshToken;

    @JsonProperty("refreshTokenExpiresAt")
    private Instant refreshTokenExpiresAt;

    @JsonProperty("updatedAt")
    private String updatedAt;

    @JsonProperty("createdAt")
    private String createdAt;

    @JsonProperty("token")
    private String token;

    public OAuthDto() {}

    public OAuthDto(String id, String status, String oauthClientId, String merchantId, String createdById, String updatedById, String ownerId, Instant expiresAt, String refreshToken, Instant refreshTokenExpiresAt, String updatedAt, String createdAt, String token) {
        this._id = id;
        this.status = status;
        this.oauthClientId = oauthClientId;
        this.merchantId = merchantId;
        this.createdById = createdById;
        this.updatedById = updatedById;
        this.ownerId = ownerId;
        this.expiresAt = expiresAt;
        this.refreshToken = refreshToken;
        this.refreshTokenExpiresAt = refreshTokenExpiresAt;
        this.updatedAt = updatedAt;
        this.createdAt = createdAt;
        this.token = token;
    }

    public String getId() {
        return _id;
    }

    public void setId(String id) {
        this._id = id;
    }

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
    }

    public String getOauthClientId() {
        return oauthClientId;
    }

    public void setOauthClientId(String oauthClientId) {
        this.oauthClientId = oauthClientId;
    }

    public String getMerchantId() {
        return merchantId;
    }

    public void setMerchantId(String merchantId) {
        this.merchantId = merchantId;
    }

    public String getCreatedById() {
        return createdById;
    }

    public void setCreatedById(String createdById) {
        this.createdById = createdById;
    }

    public String getUpdatedById() {
        return updatedById;
    }

    public void setUpdatedById(String updatedById) {
        this.updatedById = updatedById;
    }

    public String getOwnerId() {
        return ownerId;
    }

    public void setOwnerId(String ownerId) {
        this.ownerId = ownerId;
    }

    public Instant getExpiresAt() {
        return expiresAt;
    }

    public void setExpiresAt(Instant expiresAt) {
        this.expiresAt = expiresAt;
    }

    public String getRefreshToken() {
        return refreshToken;
    }

    public void setRefreshToken(String refreshToken) {
        this.refreshToken = refreshToken;
    }

    public Instant getRefreshTokenExpiresAt() {
        return refreshTokenExpiresAt;
    }

    public void setRefreshTokenExpiresAt(Instant refreshTokenExpiresAt) {
        this.refreshTokenExpiresAt = refreshTokenExpiresAt;
    }

    public String getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(String updatedAt) {
        this.updatedAt = updatedAt;
    }

    public String getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(String createdAt) {
        this.createdAt = createdAt;
    }

    public String getToken() {
        return token;
    }

    public void setToken(String token) {
        this.token = token;
    }
}
