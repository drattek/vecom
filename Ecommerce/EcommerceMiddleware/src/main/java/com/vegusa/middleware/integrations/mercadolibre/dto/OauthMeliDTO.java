package com.vegusa.middleware.integrations.mercadolibre.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class OauthMeliDTO {
    @JsonProperty("access_token")
    private String accessToken;

    @JsonProperty("expires_in")
    private Long expiresIn;

    @JsonProperty("scope")
    private String scope;

    @JsonProperty("user_id")
    private String userId;

    @JsonProperty("refresh_token")
    private String refreshToken;

    public OauthMeliDTO() {}

    public OauthMeliDTO(String accessToken, Long expiresIn, String scope, String userId, String refreshToken) {
        this.accessToken = accessToken;
        this.expiresIn = expiresIn;
        this.scope = scope;
        this.userId = userId;
        this.refreshToken = refreshToken;
    }

    public String getAccessToken() {
        return accessToken;
    }

    public void setAccessToken(String accessToken) {
        this.accessToken = accessToken;
    }

    public Long getExpiresIn() {
        return expiresIn;
    }

    public void setExpiresIn(Long expiresIn) {
        this.expiresIn = expiresIn;
    }

    public String getScope() {
        return scope;
    }

    public void setScope(String scope) {
        this.scope = scope;
    }

    public String getUserId() {
        return userId;
    }

    public void setUserId(String userId) {
        this.userId = userId;
    }

    public String getRefreshToken() {
        return refreshToken;
    }

    public void setRefreshToken(String refreshToken) {
        this.refreshToken = refreshToken;
    }
}
