package com.vegusa.veg_mv_integration_midd.oauth2_0.controller;

import com.vegusa.veg_mv_integration_midd.oauth2_0.encrypt_decrypt.AESEncryptDecrypt;
import com.vegusa.veg_mv_integration_midd.oauth2_0.service.AuthService;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.veg_mv_integration_midd.oauth2_0.utils.AuthUtils;
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
public class AuthController  implements ApplicationRunner
{
    private final AuthService authService;
    private int attemptsToeGetRefreshToken = 1;

    @Autowired
    public AuthController(AuthService authService)
    {
        this.authService = authService;
    }

    @Override
    public void run(ApplicationArguments args) throws Exception
    {
        System.out.println("Application Oauth2.0 Starts!");
        generateToken(1);
    }

    private void generateToken(int attemptNumber)
            throws InvalidAlgorithmParameterException, IllegalBlockSizeException, NoSuchPaddingException,
            NoSuchAlgorithmException, BadPaddingException, InvalidKeyException,
            JsonProcessingException, ParseException, InterruptedException
    {
        ObjectMapper objectMapper = new ObjectMapper();
        JsonNode jsonNode = objectMapper.readTree(authService.fetchAccessToken());

        if(jsonNode.has("error"))
        {
            System.out.println("Error generating Token Info: " + jsonNode.get("error").asText());

            if(attemptNumber < AuthUtils.getMaxAttemptsToGetAccessToken())
            {
                Thread.sleep(AuthUtils.getThreadSleepErrorAccessToken());
                generateToken(attemptNumber + 1);
            }
        }
        else
        {
            System.out.println("Token Info successfully obtained!");
            authService.setEncryptDecryptInterface(new AESEncryptDecrypt());
            authService.saveTokenInfo(jsonNode);
        }
    }

    @Scheduled(fixedRate = 600000)
    public void refreshTokenPeriodically()
            throws InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException,
            NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException,
            InterruptedException, ParseException
    {
        ObjectMapper objectMapper = new ObjectMapper();
        JsonNode jsonNode = objectMapper.readTree(this.authService.refreshAccessToken());
        if(jsonNode.has("error"))
        {
            System.out.println("Refresh token periodically method: " + jsonNode.get("error").asText());

            if(attemptsToeGetRefreshToken < AuthUtils.getMaxAttemptsToGetRefreshToken())
            {
                attemptsToeGetRefreshToken++;
                Thread.sleep(AuthUtils.getThreadSleepErrorRefreshToken());
                refreshTokenPeriodically();
            }
        }
        else
        {
            System.out.println("Refresh token periodically method: Access token refreshed successfully.");
            this.authService.saveTokenInfo(jsonNode);
        }
    }

}
