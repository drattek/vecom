package com.vegusa.middleware.integrations.multivende.dto;

import com.fasterxml.jackson.annotation.JsonInclude;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class OAuthDto {
    private String id;
    private String status;
    private String oauthClientId;
    private String merchantId;
    private String createdById;
    private String updatedById;
    private String ownerId;
    private String expiresAt;
    private String refreshToken;
    private String refreshTokenExpiresAt;
    private String updatedAt;
    private String createdAt;
    private String token;
}
