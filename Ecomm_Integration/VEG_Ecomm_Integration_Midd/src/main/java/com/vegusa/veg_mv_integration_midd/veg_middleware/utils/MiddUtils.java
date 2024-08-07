package com.vegusa.veg_mv_integration_midd.veg_middleware.utils;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.veg_mv_integration_midd.oauth2_0.encrypt_decrypt.AESEncryptDecrypt;
import com.vegusa.veg_mv_integration_midd.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.TokenInfo;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.TokenInfoRepository;
import org.json.JSONObject;
import org.springframework.core.env.Environment;
import org.springframework.http.HttpHeaders;
import org.springframework.web.reactive.function.client.WebClient;
import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import javax.crypto.SecretKey;
import javax.crypto.spec.IvParameterSpec;
import javax.crypto.spec.SecretKeySpec;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.util.Base64;

public class MiddUtils {
    private MiddUtils(){}

    public static String getAppInfo(WebClient webClient, String url, String accessToken) {
        try {
            HttpHeaders headers = new HttpHeaders();
            headers.add("Authorization", "Bearer " + accessToken);

            return webClient.get()
                    .uri(url)
                    .headers(h -> h.addAll(headers))
                    .retrieve()
                    .bodyToMono(String.class)
                    .block();
        } catch (RuntimeException e) {
            return MiddUtils.getSimpleJSONResponse("error", e.getMessage());
        }
    }

    public static String getDecryptedAccessToken(TokenInfoRepository tokenInfoRepository, EncryptDecryptInterface encryptDecryptInterface, Environment env, String algorithm)
            throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException, NoSuchAlgorithmException,
            BadPaddingException, InvalidKeyException {
        TokenInfo tokenInfo =  tokenInfoRepository.getTokenInfo(env.getProperty("integration.company.name"));
        if(tokenInfo != null) {
            SecretKey key = MiddUtils.convertStringToSecretKey(tokenInfo.getSecretKey());
            IvParameterSpec ivParameterSpec = MiddUtils.convertStringToIvParameterSpec(tokenInfo.getInitializationVector());
            return encryptDecryptInterface.decrypt(algorithm, tokenInfo.getCipherAccessToken(), key, ivParameterSpec);
        } else {
            throw new RuntimeException("Access token was not found in Middleware data base.");
        }
    }

    public static String getDecryptedAccessToken(TokenInfoRepository tokenInfoRepository, EncryptDecryptInterface encryptDecryptInterface, Environment env, String algorithm, String ExcMessage) {
        try {
            TokenInfo tokenInfo =  tokenInfoRepository.getTokenInfo(env.getProperty("integration.company.name"));
            if(tokenInfo != null) {
                SecretKey key = MiddUtils.convertStringToSecretKey(tokenInfo.getSecretKey());
                IvParameterSpec ivParameterSpec = MiddUtils.convertStringToIvParameterSpec(tokenInfo.getInitializationVector());
                return encryptDecryptInterface.decrypt(algorithm, tokenInfo.getCipherAccessToken(), key, ivParameterSpec);
            }
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                 NoSuchAlgorithmException | BadPaddingException | InvalidKeyException e){
            System.err.println(ExcMessage);
        }
        return "";
    }


    public static SecretKey convertStringToSecretKey(String encodedKey) {
        byte[] decodedKey = Base64.getDecoder().decode(encodedKey);
        return new SecretKeySpec(decodedKey, 0, decodedKey.length, "AES");
    }

    public static IvParameterSpec convertStringToIvParameterSpec(String encodedKey) {
        byte[] decodedKey = Base64.getDecoder().decode(encodedKey);
        return new IvParameterSpec(decodedKey);
    }

    public static EncryptDecryptInterface getEncryptDecryptInterface() {
        return new AESEncryptDecrypt();
    }

    public static JsonNode validateResponse(String errorMessage, String response) throws JsonProcessingException {
        ObjectMapper objMapper = new ObjectMapper();
        JsonNode jsonNode = objMapper.readTree(response);
        if(jsonNode.has("error")) {
            throw new RuntimeException(errorMessage + jsonNode.get("error").asText());
        } else {
            return jsonNode;
        }
    }

    public static String getSimpleJSONResponse(String key, String value){
        JSONObject response = new JSONObject();
        response.put(key, value);
        return response.toString();
    }


}
