package com.vegusa.middleware.integrations.camso.oauth;

import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.dto.IntegrationTokenRequest;
import com.vegusa.middleware.entity.IntegrationToken;
import com.vegusa.middleware.integrations.camso.dto.OauthCamsoDTO;
import com.vegusa.middleware.oauth.TokenAbstract;
import com.vegusa.middleware.repository.local.IntegrationParameterRepository;
import com.vegusa.middleware.repository.local.IntegrationTokenRepository;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import java.time.ZonedDateTime;

@Component
public class TokenStorageCamso extends TokenAbstract<OauthCamsoDTO> {
    private volatile long ID;
    private volatile String storeUrl;
    private volatile String clientSecret;

    @Autowired
    private IntegrationParameterRepository integrationParameterRepository;
    @Autowired
    private IntegrationTokenRepository integrationTokenRepository;

    public TokenStorageCamso() {}

    @PostConstruct
    public void run(){
        integrationParameterRepository.findByIntegrationName(getIntegrationName())
                .ifPresent(parameter -> {
                    this.ID = parameter.getId();
                    this.storeUrl = parameter.getStoreUrl();
                    this.clientSecret = parameter.getClientSecret();
                });
    }

    @Override
    public String getIntegrationName(){
        return IntegrationType.CAMSO.name();
    }

    @Override
    public void save(IntegrationTokenRequest<OauthCamsoDTO> request){
        OauthCamsoDTO token_data = request.getData();
        IntegrationToken token = integrationTokenRepository.findByIntegrationName(getIntegrationName())
                .orElseGet(() -> {
                    IntegrationToken new_token = new IntegrationToken();
                    new_token.setIntegrationName(getIntegrationName());
                    new_token.setIntegrationParameter(this.ID);
                    return new_token;
                });
        ZonedDateTime expirationTime = ZonedDateTime.now().plusHours(5);

        token.setAccountId(token_data.getId());
        token.setAccessToken(token_data.getAccessToken());
        token.setScope(token_data.getScope());
        token.setExpiresIn(token_data.getIssuedAt());
        token.setExpirationTime(expirationTime.toInstant());
        token.setExpirationRefreshIn(expirationTime.toInstant());
        token.setExpiresRefreshIn(token_data.getIssuedAt());

        this.setAccountId(token_data.getId());
        this.setAccessToken(token_data.getAccessToken());
        this.setExpiresIn(token_data.getIssuedAt());
        this.setExpiresRefreshIn(token_data.getIssuedAt());
        this.setExpirationTime(expirationTime.toInstant());
        this.setExpirationRefreshTime(expirationTime.toInstant());

        integrationTokenRepository.save(token);
    }

    public long getID() {
        return ID;
    }

    public void setID(long ID) {
        this.ID = ID;
    }

    public String getStoreUrl() {
        return storeUrl;
    }

    public void setStoreUrl(String storeUrl) {
        this.storeUrl = storeUrl;
    }

    public String getClientSecret() {
        return clientSecret;
    }

    public void setClientSecret(String clientSecret) {
        this.clientSecret = clientSecret;
    }
}
