package com.vegusa.middleware.integrations.mercadolibre.oauth;

import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.dto.IntegrationTokenRequest;
import com.vegusa.middleware.entity.IntegrationParameter;
import com.vegusa.middleware.entity.IntegrationToken;
import com.vegusa.middleware.integrations.mercadolibre.dto.OauthMeliDTO;
import com.vegusa.middleware.oauth.TokenAbstract;
import com.vegusa.middleware.repository.IntegrationParameterRepository;
import com.vegusa.middleware.repository.IntegrationTokenRepository;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import java.time.Instant;
import java.time.temporal.ChronoUnit;

@Component
public class TokenStorageMeli extends TokenAbstract<OauthMeliDTO> {
    private volatile IntegrationParameter ID;
    private volatile String clientId;
    private volatile String clientSecret;
    private volatile String storeUrl;
    private volatile Instant createdAtIntegration;
    private volatile Instant updatedAtIntegration;

    @Autowired
    private IntegrationParameterRepository integrationParameterRepository;

    @Autowired
    IntegrationTokenRepository integrationTokenRepository;

    @PostConstruct
    public void run(){
        integrationParameterRepository.findByIntegrationName(getIntegrationName())
                .ifPresent(parameter -> {
                    this.clientId = parameter.getClientId();
                    this.clientSecret = parameter.getClientSecret();
                    this.storeUrl = parameter.getStoreUrl();
                    this.createdAtIntegration = parameter.getCreatedAt();
                    this.updatedAtIntegration = parameter.getUpdatedAt();
                    this.ID = parameter;
                });
    }

    @Override
    public String getIntegrationName(){
        return IntegrationType.MERCADO_LIBRE.name();
    }

    @Override
    public void save(IntegrationTokenRequest<OauthMeliDTO> request){
        OauthMeliDTO token_data = request.getData();
        IntegrationToken token = integrationTokenRepository.findByIntegrationName(getIntegrationName())
                .orElseGet(() -> {
                    IntegrationToken new_token = new IntegrationToken();
                    new_token.setIntegrationName(getIntegrationName());
                    new_token.setIntegrationParameter(this.ID);

                    return new_token;
                });

        token.setAccountId(token_data.getUserId());
        token.setAccessToken(token_data.getAccessToken());
        token.setRefreshToken(token_data.getRefreshToken());
        token.setExpiresIn(token_data.getExpiresIn());
        token.setExpirationTime(Instant.now().plusMillis(token_data.getExpiresIn()));
        token.setExpirationRefreshIn(Instant.now().plus(3, ChronoUnit.MONTHS));
        token.setScope(token_data.getScope());

        integrationTokenRepository.save(token);
    }

    public String getClientId() {
        return clientId;
    }

    public void setClientId(String clientId) {
        this.clientId = clientId;
    }

    public String getClientSecret() {
        return clientSecret;
    }

    public void setClientSecret(String clientSecret) {
        this.clientSecret = clientSecret;
    }

    public String getStoreUrl() {
        return storeUrl;
    }

    public void setStoreUrl(String storeUrl) {
        this.storeUrl = storeUrl;
    }

    public Instant getCreatedAtIntegration() {
        return createdAtIntegration;
    }

    public void setCreatedAtIntegration(Instant createdAtIntegration) {
        this.createdAtIntegration = createdAtIntegration;
    }

    public Instant getUpdatedAtIntegration() {
        return updatedAtIntegration;
    }

    public void setUpdatedAtIntegration(Instant updatedAtIntegration) {
        this.updatedAtIntegration = updatedAtIntegration;
    }
}
