package com.vegusa.middleware.integrations.mercadolibre.controller;

import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.constants.TokenType;
import com.vegusa.middleware.dto.IntegrationTokenRequest;
import com.vegusa.middleware.integrations.mercadolibre.client.MercadolibreClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.OauthMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.oauth.TokenStorageMeli;
import com.vegusa.middleware.integrations.mercadolibre.utils.CodeVerifier;
import com.vegusa.middleware.repository.IntegrationParameterRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.servlet.view.RedirectView;

import java.text.MessageFormat;
import java.util.HashMap;

@RestController
@RequestMapping("msb-ecommerce-middleware/mercadolibre")
public class OauthMeliController {

    @Autowired
    private TokenStorageMeli tokenStorage;

    @Autowired
    private MercadolibreClient client;

    @Autowired
    private CodeVerifier codeVerifier;

    @Autowired
    private IntegrationParameterRepository integrationParameterRepository;

    @GetMapping(value = "/")
    public RedirectView authenticationMeli(){
        String code_verifier = codeVerifier.getCodeVerififer();
        String code_challenge = codeVerifier.getCodeChallenge(code_verifier);
        String base_url = "https://auth.mercadolibre.com.mx/authorization?response_type=code&client_id={0}&redirect_uri={1}&code_challenge={2}&code_challenge_method={3}";
        String redirect_uri = MessageFormat.format(base_url, tokenStorage.getClientId(), tokenStorage.getStoreUrl(), code_challenge, "S256");
        System.out.println(redirect_uri);

        integrationParameterRepository.findByIntegrationName(IntegrationType.MERCADO_LIBRE.name()).ifPresent(parameter -> {
            parameter.setVerifier(code_verifier);
            integrationParameterRepository.save(parameter);
        });

        RedirectView redirect = new RedirectView();
        redirect.setUrl(redirect_uri);

        return redirect;
    }

    @GetMapping(value = "/callback")
    public String meliCallback(@RequestParam("code") String code){
        System.out.println("code: " + code);

        client.performAccessToken(code).block();

        return "";
    }

    @PostMapping(value = "/refresh-token")
    public void refreshToken(){
        System.out.println("Refreshing token");
        client.performRefreshToken().block();
    }

    @PostMapping(value = "/notification")
    public void meliNotification(@RequestBody HashMap<String, String> request){}

    @PostMapping(value = "/add-token")
    public void addToken(@RequestBody HashMap<String, String> request){
        OauthMeliDTO oauth = new OauthMeliDTO();
        oauth.setAccessToken(request.get("token"));
        oauth.setRefreshToken(request.get("refresh_token"));
        oauth.setExpiresIn(Long.valueOf(request.get("expires_in")));
        oauth.setScope(request.get("scope"));
        oauth.setUserId(request.get("user_id"));

        IntegrationTokenRequest<OauthMeliDTO> token = new IntegrationTokenRequest<>();
        token.setIntegrationName(tokenStorage.getIntegrationName());
        token.setTokenType(TokenType.BEARER);
        token.setData(oauth);

        tokenStorage.save(token);
    }
}

