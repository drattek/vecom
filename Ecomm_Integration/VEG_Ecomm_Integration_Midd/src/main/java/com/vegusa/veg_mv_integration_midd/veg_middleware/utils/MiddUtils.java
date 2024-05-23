package com.vegusa.veg_mv_integration_midd.veg_middleware.utils;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.veg_mv_integration_midd.oauth2_0.encrypt_decrypt.AESEncryptDecrypt;
import com.vegusa.veg_mv_integration_midd.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.TokenInfo;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.TokenInfoRepository;
import org.springframework.http.HttpHeaders;
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
            return "{ \"error\" : \"" + e.getMessage()  + "\" }";
        }
    }

    public static String getDecryptedAccessToken(TokenInfoRepository tokenInfoRepository, EncryptDecryptInterface encryptDecryptInterface)
            throws InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException, NoSuchAlgorithmException,
            BadPaddingException, InvalidKeyException {
        TokenInfo tokenInfo =  tokenInfoRepository.findById(1).orElse(null);
        if(tokenInfo != null) {
            SecretKey key = MiddUtils.convertStringToSecretKey(tokenInfo.getSecretKey());
            IvParameterSpec ivParameterSpec = MiddUtils.convertStringToIvParameterSpec(tokenInfo.getInitializationVector());
            String algorithm = "AES/CBC/PKCS5Padding";
            return encryptDecryptInterface.decrypt(algorithm, tokenInfo.getCipherAccessToken(), key, ivParameterSpec);
        } else {
            return "{ \"error\" : \"Access token was not found.\" }";
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

    public static EncryptDecryptInterface getEncryptDecryptInterface() {
        return new AESEncryptDecrypt();
    }

    public static JsonNode getJSONResponse(String errorMessage, String response) throws JsonProcessingException {
        ObjectMapper objMapper = new ObjectMapper();
        JsonNode jsonNodeAppInfo = objMapper.readTree(response);
        if(jsonNodeAppInfo.has("error")) {
            throw new RuntimeException(errorMessage + jsonNodeAppInfo.get("error").asText());
        } else {
            return jsonNodeAppInfo;
        }
    }


}
