package com.vegusa.middleware.dto;

import com.vegusa.middleware.constants.TokenType;

public class IntegrationTokenRequest<T> {
    private String integrationName;
    private TokenType tokenType;
    private T data;

    public String getIntegrationName() {
        return integrationName;
    }

    public void setIntegrationName(String integrationName) {
        this.integrationName = integrationName;
    }

    public TokenType getTokenType() {
        return tokenType;
    }

    public void setTokenType(TokenType tokenType) {
        this.tokenType = tokenType;
    }

    public T getData() {
        return data;
    }

    public void setData(T data) {
        this.data = data;
    }
}
