package com.vegusa.oauth2_0.controller;

import com.vegusa.oauth2_0.service.AuthService;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.middleware.repository.SystemParameterRepository;
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
    private int attemptsToGetRefreshToken = 1;
    //Middleware - Repository - msb
    private final SystemParameterRepository sysParameterRepo;

    @Autowired
    public AuthController(AuthService authService, SystemParameterRepository sysParameterRepo) {
        this.authService = authService;
        this.sysParameterRepo = sysParameterRepo;
        this.authService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface());
    }

    @Override
    public void run(ApplicationArguments args) throws Exception {
        //System.out.println("Application Oauth2.0 Starts!");
        //generateToken(1);
    }

    private void generateToken(int attemptNumber) {
        /*
        try {
            ObjectMapper objectMapper = new ObjectMapper();
            JsonNode jsonNode = objectMapper.readTree(authService.fetchAccessToken());
            if(jsonNode.has("error")) {
                System.out.println("Error generating Token Info: " + jsonNode.get("error").asText());
                if(attemptNumber < sysParameterRepo
                        .getSystemParameter("ATTEMPTS_ACCESS_TOKEN").getIntValue()) {
                    Thread.sleep(sysParameterRepo
                            .getSystemParameter("ATTEMPT_ACCESS_TOKEN_SLEEP_VALUE").getIntValue());
                    generateToken(attemptNumber + 1);
                }
            } else {
                System.out.println("Token Info successfully obtained!");
                authService.saveTokenInfo(jsonNode);
            }
        } catch (JsonProcessingException | InterruptedException | InvalidAlgorithmParameterException | NoSuchPaddingException |
                IllegalBlockSizeException | NoSuchAlgorithmException |  BadPaddingException | ParseException | InvalidKeyException e) {
            System.err.println("An error occurred while saving the token.");
            System.err.println("StackTrace: ");
        }
         */
    }

    //@Scheduled(fixedRateString = "${fixedRateRefreshToken.in.milliseconds}", initialDelayString = "3000")
    public void refreshAccessTokenPeriodically() {
        try {
            ObjectMapper objectMapper = new ObjectMapper();
            JsonNode jsonNode = objectMapper.readTree(this.authService.refreshAccessToken());
            if(jsonNode.has("error")) {
                System.err.println("Refresh token periodically method: " + jsonNode.get("error").asText());
                if(attemptsToGetRefreshToken < sysParameterRepo
                        .getSystemParameter("ATTEMPTS_REFRESH_TOKEN").getIntValue()) {
                    attemptsToGetRefreshToken++;
                    Thread.sleep(sysParameterRepo
                            .getSystemParameter("ATTEMPT_REFRESH_TOKEN_SLEEP_VALUE").getIntValue());
                    refreshAccessTokenPeriodically();
                }
            } else {
                System.out.println("Access Token refreshed successfully.");
                this.authService.saveTokenInfo(jsonNode);
            }
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException | NoSuchAlgorithmException |
                BadPaddingException | InvalidKeyException | JsonProcessingException | InterruptedException | ParseException e){
            System.err.println("An error occurred while refreshing the token.");
        }
    }

    //@Scheduled(fixedRateString = "${fixedRateRefreshToken.in.milliseconds}", initialDelayString = "${fixedDelayRefreshAuthInfo.in.milliseconds}")
    public void refreshAuthTokenInfoPeriodically(){
        try {
            this.authService.refreshAuthToken();
        } catch (RuntimeException e){
            System.err.println("An error occurred while refreshing Auth Token Info: " + e.getMessage());
        }
    }





}
