package com.vegusa.middleware.integrations.mercadolibre.oauth;

import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.dto.IntegrationTokenRequest;
import com.vegusa.middleware.entity.IntegrationToken;
import com.vegusa.middleware.integrations.mercadolibre.dto.OauthMeliDTO;
import com.vegusa.middleware.oauth.TokenAbstract;
import com.vegusa.middleware.repository.IntegrationParameterRepository;
import com.vegusa.middleware.repository.IntegrationTokenRepository;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import java.time.Duration;
import java.time.Instant;
import java.time.ZonedDateTime;
import java.time.temporal.ChronoUnit;

@Component
public class TokenStorageMeli extends TokenAbstract<OauthMeliDTO> {
    private volatile long ID;
    private volatile String siteId;
    private volatile String clientId;
    private volatile String clientSecret;
    private volatile String storeUrl;
    private volatile Instant createdAtIntegration;
    private volatile Instant updatedAtIntegration;

    @Autowired
    private IntegrationParameterRepository integrationParameterRepository;

    @Autowired
    IntegrationTokenRepository integrationTokenRepository;

    public TokenStorageMeli() {}

    @PostConstruct
    public void run(){
        integrationParameterRepository.findByIntegrationName(getIntegrationName())
                .ifPresent(parameter -> {
                    this.clientId = parameter.getClientId();
                    this.clientSecret = parameter.getClientSecret();
                    this.storeUrl = parameter.getStoreUrl();
                    this.createdAtIntegration = parameter.getCreatedAt();
                    this.updatedAtIntegration = parameter.getUpdatedAt();
                    this.ID = parameter.getId();
                    this.siteId = parameter.getSiteId();
                });
    }

    @Override
    public String getIntegrationName(){
        return IntegrationType.MERCADO_LIBRE.name();
    }

    public long getID() {
        return ID;
    }

    public void setID(long ID) {
        this.ID = ID;
    }

    public String getSiteId() {
        return siteId;
    }

    public void setSiteId(String siteId) {
        this.siteId = siteId;
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
        ZonedDateTime expirationTime = ZonedDateTime.now().plusHours(5);
        ZonedDateTime expirationRefreshTime = ZonedDateTime.now().plusMonths(3);

        token.setAccountId(token_data.getUserId());
        token.setAccessToken(token_data.getAccessToken());
        token.setRefreshToken(token_data.getRefreshToken());
        token.setExpiresIn(token_data.getExpiresIn());
        token.setExpiresRefreshIn(token_data.getExpiresIn());
        token.setExpirationTime(expirationTime.toInstant());
        token.setExpirationRefreshIn(expirationRefreshTime.toInstant());
        token.setScope(token_data.getScope());

        this.setAccountId(token_data.getUserId());
        this.setAccessToken(token_data.getAccessToken());
        this.setRefreshToken(token_data.getRefreshToken());
        this.setExpiresIn(token_data.getExpiresIn());
        this.setExpiresRefreshIn(token_data.getExpiresIn());
        this.setExpirationTime(expirationTime.toInstant());
        this.setExpirationRefreshTime(expirationRefreshTime.toInstant());

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
