package com.vegusa.veg_mv_integration_midd.oauth2_0.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.vegusa.veg_mv_integration_midd.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.TokenInfo;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.TokenInfoParameters;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.TokenInfoParametersRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.TokenInfoRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.VegMvIntegrationEndptsRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.utils.MiddUtils;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.core.env.Environment;
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
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.text.ParseException;
import java.text.SimpleDateFormat;
import java.util.Base64;

@Service
public class AuthService {
    private final WebClient webClient;
    //Middleware - Repository
    private final TokenInfoRepository tokenInfoRepository;
    private final TokenInfoParametersRepository tokenInfoParametersRepository;
    private final VegMvIntegrationEndptsRepository endptsRepository;
    private EncryptDecryptInterface encryptDecryptInterface;
    private final Environment env;

    @Autowired
    public AuthService(WebClient webClient,
                       TokenInfoRepository tokenInfoRepository,
                       TokenInfoParametersRepository tokenInfoParametersRepository,
                       VegMvIntegrationEndptsRepository endptsRepository,
                       Environment env) {
        this.webClient = webClient;
        this.tokenInfoRepository = tokenInfoRepository;
        this.tokenInfoParametersRepository = tokenInfoParametersRepository;
        this.endptsRepository = endptsRepository;
        this.env = env;
    }

    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface) {
        this.encryptDecryptInterface = encryptDecryptInterface;
    }

    private MultiValueMap<String, String> getTokenInfoParameters(String origin) {
        MultiValueMap<String, String> bodyValues = new LinkedMultiValueMap<>();
        TokenInfoParameters tokenInfoParameters = tokenInfoParametersRepository
                .getTokenInfoParameters(env.getProperty("integration.company.name"));

        if(tokenInfoParameters != null) {
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
        } else {
            System.err.println("No parameters were found to obtain the access token with grant type: " + origin + " and company name: " +
                    env.getProperty("integration.company.name"));
        }
        return  bodyValues;
    }
    public String fetchAccessToken() {
        try {
            return webClient.post()
                    .uri(endptsRepository.getEndPointMuitiVende("AUTHENTICATE_OAUTH2"))
                    .body(BodyInserters.fromFormData(getTokenInfoParameters("authorization_code")))
                    .retrieve()
                    .bodyToMono(String.class)
                    .block();
        } catch (WebClientResponseException e) {
            return "{ \"error\" : \"" + e.getStatusCode() + " " + e.getMessage()  + "\" }";
        }
    }

    public void saveTokenInfo(JsonNode jsonNode)
            throws NoSuchAlgorithmException, InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException,
                    BadPaddingException, InvalidKeyException, ParseException {
        SecretKey key = encryptDecryptInterface.generateKey(128);
        IvParameterSpec ivParameterSpec = encryptDecryptInterface.generateIv();
        String algorithm = "AES/CBC/PKCS5Padding";
        String cipherAccessToken = encryptDecryptInterface.encrypt(algorithm, jsonNode.get("token").asText(), key, ivParameterSpec);
        String cipherRefreshToken = encryptDecryptInterface.encrypt(algorithm, jsonNode.get("refreshToken").asText(), key, ivParameterSpec);
        SimpleDateFormat formatter = new SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss.SSS'Z'");

        TokenInfo auxTokenInfo = tokenInfoRepository.getTokenInfo(env.getProperty("integration.company.name"));
        TokenInfo tokenInfo =  (auxTokenInfo != null) ? auxTokenInfo : new TokenInfo();

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
        tokenInfo.setIntegrationCompany(env.getProperty("integration.company.name"));
        tokenInfoRepository.save(tokenInfo);
        System.out.println("Token Info saved successfully.");
    }

    private static String convertSecretKeyToString(SecretKey secretKey) {
        byte[] rawData = secretKey.getEncoded();
        return Base64.getEncoder().encodeToString(rawData);
    }

    private static String convertIvParameterSpecToString(IvParameterSpec ivParameterSpec) {
        byte[] rawData = ivParameterSpec.getIV();
        return Base64.getEncoder().encodeToString(rawData);
    }

    public String refreshAccessToken()
            throws InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException, NoSuchAlgorithmException,
                    BadPaddingException, InvalidKeyException, JsonProcessingException {
        TokenInfo tokenInfo = tokenInfoRepository.getTokenInfo(env.getProperty("integration.company.name"));
        if(tokenInfo != null) {
            SecretKey key = MiddUtils.convertStringToSecretKey(tokenInfo.getSecretKey());
            IvParameterSpec ivParameterSpec = MiddUtils.convertStringToIvParameterSpec(tokenInfo.getInitializationVector());
            String algorithm = "AES/CBC/PKCS5Padding";
            String refreshToken = encryptDecryptInterface.decrypt(algorithm, tokenInfo.getCipherRefreshToken(), key, ivParameterSpec);

            MultiValueMap<String, String> bodyValues = getTokenInfoParameters("refresh_token");
            bodyValues.add("refresh_token", refreshToken);

            try {
                return webClient.post()
                        .uri(endptsRepository.getEndPointMuitiVende("REFRESH_TOKEN_OAUTH2"))
                        .body(BodyInserters.fromFormData(bodyValues))
                        .retrieve()
                        .bodyToMono(String.class)
                        .block();
            } catch (WebClientResponseException e) {
                return "{ \"error\" : \"" + e.getStatusCode() + " " + e.getMessage()  + "\" }";
            }
        } else {
            return "{ \"error\" : \"Token info to generate the refresh token was not found.\" }";
        }
    }

}
