package com.vegusa.middleware.integrations.camso.controller.common;

import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.entity.IntegrationToken;
import com.vegusa.middleware.integrations.camso.client.CamsoClient;
import com.vegusa.middleware.integrations.camso.dto.OauthCamsoDTO;
import com.vegusa.middleware.repository.IntegrationTokenRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

import java.time.Instant;
import java.util.HashMap;

@RestController
@RequestMapping("/msb-ecommerce-middleware/camso")
public class CommonCamsoController {
    @Autowired
    private IntegrationTokenRepository integrationTokenRepository;

    @Autowired
    private CamsoClient client;

    @PostMapping(value = "/set-data")
    public void setData(@RequestBody HashMap<String, String> request){
        IntegrationToken token = integrationTokenRepository.findByIntegrationName(IntegrationType.CAMSO.name())
                .orElseGet(() -> {
                    IntegrationToken newToken = new IntegrationToken();
                    newToken.setIntegrationName(IntegrationType.CAMSO.name());
                    newToken.setIntegrationParameter(3L);

                    return newToken;
                });
        System.out.println("access_token: " + request.get("access_token"));
        token.setAccountId(request.get("id"));
        token.setAccessToken(request.get("access_token"));
        token.setScope(request.get("scope"));
        token.setExpiresIn(Long.valueOf(request.get("issued_at")));
        token.setExpiresRefreshIn(Long.valueOf(request.get("issued_at")));
        token.setApiKey("Yz6DjdQCFiDG9O21NhxlTLhqoQMZCcbx");

        Instant expiresIn = Instant.now();
        token.setExpirationTime(expiresIn);
        token.setExpirationRefreshIn(expiresIn);

        token.setCreatedAt(Instant.now());
        token.setUpdatedAt(Instant.now());

        integrationTokenRepository.save(token);
    }

    @PostMapping(value = "/get-token")
    public Mono<ResponseEntity<String>> getToken(){
        client.performAccessToken().subscribe();

        return Mono.just(ResponseEntity.accepted().body("Token requested"));
    }
}
