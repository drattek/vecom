package com.vegusa.oauth2_0.controller;

import com.vegusa.oauth2_0.service.AuthService;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.middleware.repository.msb.VegEcommGralParameterRepository;
import com.vegusa.middleware.utils.MWUtils;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.text.ParseException;

@Component
public class AuthController implements ApplicationRunner {
    private final AuthService authService;
    private int attemptsToeGetRefreshToken = 1;
    //Middleware - Repository - msb
    private final VegEcommGralParameterRepository vegEcommGralParameterRepository;

    @Autowired
    public AuthController(AuthService authService, VegEcommGralParameterRepository vegEcommGralParameterRepository)
    {
        this.authService = authService;
        this.vegEcommGralParameterRepository = vegEcommGralParameterRepository;
        this.authService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface());
    }

    @Override
    public void run(ApplicationArguments args) throws Exception {
        System.out.println("Application Oauth2.0 Starts!");
        generateToken(1);
    }

    private void generateToken(int attemptNumber) {
        try {
            ObjectMapper objectMapper = new ObjectMapper();
            JsonNode jsonNode = objectMapper.readTree(authService.fetchAccessToken());
            if(jsonNode.has("error")) {
                System.out.println("Error generating Token Info: " + jsonNode.get("error").asText());
                if(attemptNumber < vegEcommGralParameterRepository
                        .getMiddlewareGeneralParameter("ATTEMPS_ACCESS_TOKEN").getIntValue()) {
                    Thread.sleep(vegEcommGralParameterRepository
                            .getMiddlewareGeneralParameter("ATTEMP_ACCESS_TOKEN_SLEEP_VALUE").getIntValue());
                    generateToken(attemptNumber + 1);
                }
            } else {
                System.out.println("Token Info successfully obtained!");
                authService.saveTokenInfo(jsonNode);
            }
        }catch (JsonProcessingException | InterruptedException | InvalidAlgorithmParameterException | NoSuchPaddingException |
                IllegalBlockSizeException | NoSuchAlgorithmException |  BadPaddingException | ParseException | InvalidKeyException e) {
            System.err.println("An error occurred while saving the token.");
            System.err.println("StackTrace: ");
        }

    }

    @Scheduled(fixedRateString = "${fixedRateRefreshToken.in.milliseconds}", initialDelayString = "${fixedDelayRefreshToken.in.milliseconds}")
    public void refreshTokenPeriodically() {
        try {
            ObjectMapper objectMapper = new ObjectMapper();
            JsonNode jsonNode = objectMapper.readTree(this.authService.refreshAccessToken());
            if(jsonNode.has("error")) {
                System.err.println("Refresh token periodically method: " + jsonNode.get("error").asText());
                if(attemptsToeGetRefreshToken < vegEcommGralParameterRepository
                        .getMiddlewareGeneralParameter("ATTEMPS_REFRESH_TOKEN").getIntValue()) {
                    attemptsToeGetRefreshToken++;
                    Thread.sleep(vegEcommGralParameterRepository
                            .getMiddlewareGeneralParameter("ATTEMP_REFRESH_TOKEN_SLEEP_VALUE").getIntValue());
                    refreshTokenPeriodically();
                }
            } else {
                System.out.println("Refresh token periodically method: Access token refreshed successfully.");
                this.authService.saveTokenInfo(jsonNode);
            }
        } catch (InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException | NoSuchAlgorithmException |
                BadPaddingException | InvalidKeyException | JsonProcessingException | InterruptedException | ParseException e){
            System.err.println("An error occurred while refreshing the token.");
            System.err.println("StackTrace: ");
        }
    }

}
