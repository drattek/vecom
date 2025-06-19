package com.vegusa.oauth2_0.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.vegusa.middleware.entity.AuthToken;
import com.vegusa.middleware.repository.AuthTokenRepository;
import com.vegusa.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.middleware.entity.AuthTokenParameter;
import com.vegusa.middleware.repository.AuthTokenParameterRepository;
import com.vegusa.middleware.repository.EndpointRepository;
import com.vegusa.middleware.utils.MWUtils;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.core.env.Environment;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.util.LinkedMultiValueMap;
import org.springframework.util.MultiValueMap;
import org.springframework.web.reactive.function.BodyInserters;
import org.springframework.web.reactive.function.client.WebClient;

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
    //Middleware - Repository
    private final AuthTokenParameterRepository authTokenParametersRepo;
    private final AuthTokenRepository authTokenRepo;
    private final EndpointRepository endpointRepo;
    private AuthToken authToken;
    private EncryptDecryptInterface encryptDecryptInterface;
    private final Environment env;
    private final WebClient webClient;

    @Autowired
    public AuthService(AuthTokenParameterRepository authTokenParametersRepo,
                       AuthTokenRepository authTokenRepo,
                       EndpointRepository endpointRepo,
                       WebClient webClient,
                       Environment env) {
        this.authTokenRepo = authTokenRepo;
        this.authTokenParametersRepo = authTokenParametersRepo;
        this.endpointRepo = endpointRepo;
        this.webClient = webClient;
        this.env = env;
    }

    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface) {
        this.encryptDecryptInterface = encryptDecryptInterface;
    }

    public AuthToken getAuthToken(){
        return authToken;
    }

    private MultiValueMap<String, String> getTokenInfoParameters(String origin) throws RuntimeException {
        MultiValueMap<String, String> bodyValues = new LinkedMultiValueMap<>();
        AuthTokenParameter tokenInfoParameters = authTokenParametersRepo
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
                    .uri(endpointRepo.getEndpointUrl("AUTHENTICATE_OAUTH2", "MULTIVENDE"))
                    .body(BodyInserters.fromFormData(getTokenInfoParameters("authorization_code")))
                    .retrieve()
                    .bodyToMono(String.class)
                    .block();
        } catch (RuntimeException e) {
            return MWUtils.getSimpleJSONResponse("error", e.getMessage());
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

        AuthToken auxTokenInfo = authTokenRepo.getAuthToken(env.getProperty("integration.company.name"));
        AuthToken tokenInfo =  (auxTokenInfo != null) ? auxTokenInfo : new AuthToken();

        tokenInfo.setResponseId(jsonNode.get("_id").asText());
        tokenInfo.setStatus(jsonNode.get("status").asText());
        tokenInfo.setCipherAccessToken(cipherAccessToken);
        tokenInfo.setAccessTokenExpiresAt(formatter.parse(jsonNode.get("expiresAt").asText()));
        tokenInfo.setCipherRefreshToken(cipherRefreshToken);
        tokenInfo.setRefreshTokenExpiresAt(formatter.parse(jsonNode.get("refreshTokenExpiresAt").asText()));
        tokenInfo.setSecretKey(AuthService.convertSecretKeyToString(key));
        tokenInfo.setInitializationVector(AuthService.convertIvParameterSpecToString(ivParameterSpec));
        tokenInfo.setUpdatedAt(formatter.parse(jsonNode.get("updatedAt").asText()));
        tokenInfo.setCreatedAt(formatter.parse(jsonNode.get("createdAt").asText()));
        tokenInfo.setIntegrationCompany(env.getProperty("integration.company.name"));
        authTokenRepo.save(tokenInfo);
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
            throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException, NoSuchAlgorithmException,
                    BadPaddingException, InvalidKeyException, JsonProcessingException {
        AuthToken tokenInfo = authTokenRepo.getAuthToken(env.getProperty("integration.company.name"));
        if(tokenInfo != null) {
            SecretKey key = MWUtils.convertStringToSecretKey(tokenInfo.getSecretKey());
            IvParameterSpec ivParameterSpec = MWUtils.convertStringToIvParameterSpec(tokenInfo.getInitializationVector());
            String algorithm = "AES/CBC/PKCS5Padding";
            String refreshToken = encryptDecryptInterface.decrypt(algorithm, tokenInfo.getCipherRefreshToken(), key, ivParameterSpec);
            MultiValueMap<String, String> bodyValues = getTokenInfoParameters("refresh_token");
            bodyValues.add("refresh_token", refreshToken);
            try {
                return webClient.post()
                        .uri(endpointRepo.getEndpointUrl("REFRESH_TOKEN_OAUTH2", "MULTIVENDE"))
                        .body(BodyInserters.fromFormData(bodyValues))
                        .retrieve()
                        .bodyToMono(String.class)
                        .block();
            } catch (RuntimeException e) {
                return MWUtils.getSimpleJSONResponse("error", e.getMessage());
            }
        } else {
            return MWUtils.getSimpleJSONResponse("error", "Token info to generate the refresh token was not found.");
        }
    }

    public void refreshAuthToken() throws  RuntimeException {
        this.authToken = authTokenRepo.getAuthToken(env.getProperty("integration.company.name"));
        System.out.println("Auth Token Info refreshed successfully.");
    }

}
