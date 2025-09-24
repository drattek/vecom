package com.vegusa.middleware.integrations.camso.client.inventory;

import com.vegusa.middleware.integrations.camso.client.CamsoClient;
import com.vegusa.middleware.integrations.camso.dto.InventoryCamsoDTO;
import com.vegusa.middleware.integrations.camso.oauth.TokenStorageCamso;
import io.netty.handler.ssl.SslContextBuilder;
import io.netty.handler.ssl.util.InsecureTrustManagerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatusCode;
import org.springframework.http.MediaType;
import org.springframework.http.client.reactive.ReactorClientHttpConnector;
import org.springframework.stereotype.Component;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.core.publisher.Mono;
import reactor.netty.http.client.HttpClient;

import javax.net.ssl.SSLException;
import java.time.Duration;
import java.util.List;

@Component
public class InventoryCamsoClient {
    private final WebClient client;

    @Autowired
    private TokenStorageCamso tokenStorage;

    public InventoryCamsoClient(WebClient client) {
        HttpClient httpClient = HttpClient.create()
                .secure(sslContextSpec -> {
                    try {
                        sslContextSpec.sslContext(
                                SslContextBuilder.forClient()
                                        .trustManager(InsecureTrustManagerFactory.INSTANCE)
                                        .build()
                        );
                    } catch (SSLException e) {
                        throw new RuntimeException(e);
                    }
                })
                .followRedirect(true);
        this.client = WebClient.builder()
                .clientConnector(new ReactorClientHttpConnector(httpClient))
                .build();
    }

    public InventoryCamsoDTO getInventory(){
        try {
            String apiKey = tokenStorage.getApiKey();
            String token = tokenStorage.getAccessToken();
            return client.get()
                    .uri("https://api.michelin.com/mxg-inventory-v1/")
                    .headers(httpHeaders -> {
                        httpHeaders.add("apiKey", apiKey);
                        httpHeaders.add("token", token);
                        httpHeaders.setAccept(List.of(MediaType.APPLICATION_JSON));
                    })
                    .retrieve()
                    .bodyToMono(InventoryCamsoDTO.class)
                    .block();
        } catch (Exception e) {
            System.out.println("Error en getInventory: " + e.getMessage());
            return null;
        }
    }
}
