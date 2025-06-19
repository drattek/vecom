package com.vegusa.middleware.integrations.mercadolibre.controller;

import com.vegusa.middleware.integrations.mercadolibre.client.MercadolibreClient;
import com.vegusa.middleware.integrations.mercadolibre.oauth.TokenStorageMeli;
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

    @GetMapping(value = "/")
    public RedirectView authenticationMeli(){
        String base_url = "https://auth.mercadolibre.com.mx/authorization?response_type=code&client_id={0}&redirect_uri={1}";
        String redirect_uri = MessageFormat.format(base_url, tokenStorage.getClientId(), tokenStorage.getStoreUrl());
        System.out.println(redirect_uri);
        RedirectView redirect = new RedirectView();
        redirect.setUrl(redirect_uri);

        return redirect;
    }

    @GetMapping(value = "/callback")
    public String meliCallback(@RequestParam("code") String code){
        System.out.println("code: " + code);

        client.performAccessToken(code);

        return "";
    }
}

