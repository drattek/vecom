package com.vegusa.middleware.integrations.multivende.client.common;

import com.vegusa.middleware.entity.AuthTokenParameter;
import com.vegusa.middleware.integrations.multivende.client.MultivendeClient;
import com.vegusa.middleware.integrations.multivende.dto.OAuthDto;
import com.vegusa.middleware.repository.AuthTokenParameterRepository;
import com.vegusa.middleware.repository.EndpointRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;
import reactor.core.publisher.Mono;

import java.util.HashMap;
import java.util.Map;

@Component
public class MultivendeAuthentication {

    private final MultivendeClient multivendeClient;

    @Autowired
    private EndpointRepository endpointRepository;

    @Autowired
    private AuthTokenParameterRepository authTokenParameterRepository;

    public MultivendeAuthentication(MultivendeClient multivendeClient){
        this.multivendeClient = multivendeClient;
    }

    public Mono<OAuthDto> getAccessToken(){
        AuthTokenParameter authTokenParameter = authTokenParameterRepository.getTokenInfoParameters("MULTIVENDE");
        Map<String, String> bodyValues = new HashMap<>();
        bodyValues.put("client_id", authTokenParameter.getClientId());
        bodyValues.put("client_secret", authTokenParameter.getClientSecret());
        bodyValues.put("grant_type", authTokenParameter.getGrantTypeAuthCode());
        bodyValues.put("code", authTokenParameter.getAuthorizationCode());
        return multivendeClient.executeRateLimited(() ->
                multivendeClient.getClient()
                        .post()
                        .uri(endpointRepository.getEndpointUrl("AUTHENTICATE_OAUTH2", "MULTIVENDE"))
                        .bodyValue(bodyValues)
                        .retrieve()
                        .bodyToMono(OAuthDto.class)
        );
    }
}
