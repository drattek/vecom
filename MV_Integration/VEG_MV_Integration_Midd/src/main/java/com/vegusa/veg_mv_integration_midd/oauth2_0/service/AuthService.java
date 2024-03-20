package com.vegusa.veg_mv_integration_midd.oauth2_0.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.vegusa.veg_mv_integration_midd.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.TokenInfo;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.TokenInfoParameters;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.TokenInfoParametersRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.TokenInfoRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.util.LinkedMultiValueMap;
import org.springframework.util.MultiValueMap;
import org.springframework.web.reactive.function.BodyInserters;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.reactive.function.client.WebClientResponseException;
import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import javax.crypto.SecretKey;
import javax.crypto.spec.IvParameterSpec;
import javax.crypto.spec.SecretKeySpec;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.text.ParseException;
import java.text.SimpleDateFormat;
import java.util.Base64;

@Service
public class AuthService
{
    private final WebClient webClient;
    private final TokenInfoRepository tokenInfoRepository;
    private final TokenInfoParametersRepository tokenInfoParametersRepository;
    private EncryptDecryptInterface encryptDecryptInterface;

    @Autowired
    public AuthService(WebClient webClient, TokenInfoRepository tokenInfoRepository, TokenInfoParametersRepository tokenInfoParametersRepository)
    {
        this.webClient = webClient;
        this.tokenInfoRepository = tokenInfoRepository;
        this.tokenInfoParametersRepository = tokenInfoParametersRepository;
    }

    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface)
    {
        this.encryptDecryptInterface = encryptDecryptInterface;
    }

    private MultiValueMap<String, String>  getTokenInfoParameters(String origin)
    {
        MultiValueMap<String, String> bodyValues = new LinkedMultiValueMap<>();
        TokenInfoParameters tokenInfoParameters = tokenInfoParametersRepository.findById(1).orElse(null);

        if(tokenInfoParameters != null)
        {
            bodyValues.add("client_id", tokenInfoParameters.getClientId());
            bodyValues.add("client_secret", tokenInfoParameters.getClientSecret());

            switch (origin) {
                case "authorization_code":
                    bodyValues.add("grant_type", tokenInfoParameters.getGrantTypeAuthCode());
                    bodyValues.add("code", tokenInfoParameters.getAuthorizationCode());
                    break;
                case "refresh_token":
                    bodyValues.add("grant_type", tokenInfoParameters.getGrantTypeRefreshToken());
                    break;
                default:
                    // Default
            }

        }
        else
        {
            System.out.println("No parameters were found to obtain the access token with grant type: " + origin);
        }

        return  bodyValues;
    }
    public String fetchAccessToken()
    {
        try
        {
            return webClient.post()
                    .uri("https://app.multivende.com/oauth/access-token")
                    .body(BodyInserters.fromFormData(getTokenInfoParameters("authorization_code")))
                    .retrieve()
                    .bodyToMono(String.class)
                    .block();
        }
        catch (WebClientResponseException e)
        {
            return "{ \"error\" : \"" + e.getStatusCode() + " " + e.getMessage()  + "\" }";
        }
    }

    public void saveTokenInfo(JsonNode jsonNode)
            throws NoSuchAlgorithmException, IllegalBlockSizeException, InvalidKeyException,
            BadPaddingException, InvalidAlgorithmParameterException, NoSuchPaddingException, ParseException
    {
        SecretKey key = encryptDecryptInterface.generateKey(128);
        IvParameterSpec ivParameterSpec = encryptDecryptInterface.generateIv();
        String algorithm = "AES/CBC/PKCS5Padding";
        String cipherAccessToken = encryptDecryptInterface.encrypt(algorithm, jsonNode.get("token").asText(), key, ivParameterSpec);
        String cipherRefreshToken = encryptDecryptInterface.encrypt(algorithm, jsonNode.get("refreshToken").asText(), key, ivParameterSpec);
        SimpleDateFormat formatter = new SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss.SSS'Z'");

        TokenInfo tokenInfo =  tokenInfoRepository.findById(1).orElseGet(TokenInfo::new);
        tokenInfo.setIdMv(jsonNode.get("_id").asText());
        tokenInfo.setStatus(jsonNode.get("status").asText());
        tokenInfo.setCipherAccessToken(cipherAccessToken);
        tokenInfo.setAccessTokenExpiresAt(formatter.parse(jsonNode.get("refreshTokenExpiresAt").asText()));
        tokenInfo.setCipherRefreshToken(cipherRefreshToken);
        tokenInfo.setRefreshTokenExpiresAt(formatter.parse(jsonNode.get("refreshTokenExpiresAt").asText()));
        tokenInfo.setSecretKey(AuthService.convertSecretKeyToString(key));
        tokenInfo.setInitializationVector(AuthService.convertIvParameterSpecToString(ivParameterSpec));
        tokenInfo.setUpdatedAt(formatter.parse(jsonNode.get("updatedAt").asText()));
        tokenInfo.setCreatedAt(formatter.parse(jsonNode.get("createdAt").asText()));

        tokenInfoRepository.save(tokenInfo);
        System.out.println("Token Info saved successfully");
    }

    public static String convertSecretKeyToString(SecretKey secretKey) throws NoSuchAlgorithmException {
        byte[] rawData = secretKey.getEncoded();
        return Base64.getEncoder().encodeToString(rawData);
    }

    public static String convertIvParameterSpecToString(IvParameterSpec ivParameterSpec) throws NoSuchAlgorithmException {
        byte[] rawData = ivParameterSpec.getIV();
        return Base64.getEncoder().encodeToString(rawData);
    }

    public String refreshAccessToken()
            throws InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException,
            NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException
    {
        TokenInfo tokenInfo =  tokenInfoRepository.findById(1).orElse(null);

        if(tokenInfo != null)
        {
            SecretKey key = AuthService.convertStringToSecretKey(tokenInfo.getSecretKey());
            IvParameterSpec ivParameterSpec = AuthService.convertStringToIvParameterSpec(tokenInfo.getInitializationVector());
            String algorithm = "AES/CBC/PKCS5Padding";
            String refreshToken = encryptDecryptInterface.decrypt(algorithm, tokenInfo.getCipherRefreshToken(), key, ivParameterSpec);

            MultiValueMap<String, String> bodyValues = getTokenInfoParameters("refresh_token");
            bodyValues.add("refresh_token", refreshToken);

            try
            {
                return webClient.post()
                        .uri("https://app.multivende.com/oauth/access-token")
                        .body(BodyInserters.fromFormData(bodyValues))
                        .retrieve()
                        .bodyToMono(String.class)
                        .block();
            }
            catch (WebClientResponseException e)
            {
                return "{ \"error\" : \"" + e.getStatusCode() + " " + e.getMessage()  + "\" }";
            }
        }
        else
        {
            return "{ \"error\" : \"Token info to generate the refresh token was not found.\" }";
        }

    }

    public static SecretKey convertStringToSecretKey(String encodedKey) {
        byte[] decodedKey = Base64.getDecoder().decode(encodedKey);
        return new SecretKeySpec(decodedKey, 0, decodedKey.length, "AES");
    }

    public static IvParameterSpec convertStringToIvParameterSpec(String encodedKey) {
        byte[] decodedKey = Base64.getDecoder().decode(encodedKey);
        return new IvParameterSpec(decodedKey);
    }

}
