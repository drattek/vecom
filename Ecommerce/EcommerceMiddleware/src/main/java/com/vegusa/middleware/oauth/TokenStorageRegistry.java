package com.vegusa.middleware.oauth;

import com.vegusa.middleware.dto.IntegrationTokenRequest;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import java.util.HashMap;
import java.util.List;
import java.util.Map;

@Component
public class TokenStorageRegistry {

    private final Map<String, TokenAbstract<?>> registry = new HashMap<>();

    @Autowired
    public TokenStorageRegistry(List<TokenAbstract<?>> storages) {
        for (TokenAbstract<?> storage : storages) {
            registry.put(storage.getIntegrationName(), storage);
        }
    }

    @SuppressWarnings("unchecked")
    public <T> void saveToken(IntegrationTokenRequest<T> request) {
        TokenAbstract<T> storage = (TokenAbstract<T>) registry.get(request.getIntegrationName());
        if (storage == null) {
            throw new IllegalArgumentException("No TokenStorage found for integration: " + request.getIntegrationName());
        }
        storage.save(request);
    }
}

