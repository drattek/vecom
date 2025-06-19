package com.vegusa.middleware.service;

import com.vegusa.middleware.dto.IntegrationTokenRequest;
import com.vegusa.middleware.oauth.TokenStorageRegistry;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

@Service
public class TokenService {

    private final TokenStorageRegistry registry;

    @Autowired
    public TokenService(TokenStorageRegistry registry) {
        this.registry = registry;
    }

    public <T> void processToken(IntegrationTokenRequest<T> request) {
        registry.saveToken(request);
    }
}

