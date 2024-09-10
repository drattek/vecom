package com.vegusa.middleware.utils;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.oauth2_0.encrypt_decrypt.AESEncryptDecrypt;
import com.vegusa.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.middleware.entity.AuthToken;
import com.vegusa.middleware.repository.AuthTokenRepository;
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
import java.util.HashMap;

public class MWUtils {
    private MWUtils(){}

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
            throw new RuntimeException("An error occurred while obtaining App Information: " + e.getMessage());
        }
    }

    public static String getDecryptedAccessToken(AuthTokenRepository authTokenRepo, EncryptDecryptInterface encryptDecryptInterface, Environment env, String algorithm)
            throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException, NoSuchAlgorithmException,
            BadPaddingException, InvalidKeyException {
        AuthToken tokenInfo =  authTokenRepo.getAuthToken(env.getProperty("integration.company.name"));
        if(tokenInfo != null) {
            SecretKey key = MWUtils.convertStringToSecretKey(tokenInfo.getSecretKey());
            IvParameterSpec ivParameterSpec = MWUtils.convertStringToIvParameterSpec(tokenInfo.getInitializationVector());
            return encryptDecryptInterface.decrypt(algorithm, tokenInfo.getCipherAccessToken(), key, ivParameterSpec);
        } else {
            throw new RuntimeException("Authorization token was not found in Middleware data base.");
        }
    }

    public static String getDecryptedAccessToken(AuthTokenRepository tokenInfoRepository, EncryptDecryptInterface encryptDecryptInterface, Environment env, String algorithm, String ExcMessage) {
        try {
            AuthToken tokenInfo =  tokenInfoRepository.getAuthToken(env.getProperty("integration.company.name"));
            if(tokenInfo != null) {
                SecretKey key = MWUtils.convertStringToSecretKey(tokenInfo.getSecretKey());
                IvParameterSpec ivParameterSpec = MWUtils.convertStringToIvParameterSpec(tokenInfo.getInitializationVector());
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

    public static String getJsonNodeResponse(String response, String label) throws JsonProcessingException {
        ObjectMapper objMapper = new ObjectMapper();
        JsonNode jsonNode = objMapper.readTree(response);
        return jsonNode.get(label).asText();
    }

    public static String getSimpleJSONResponse(String key, String value){
        JSONObject response = new JSONObject();
        response.put(key, value);
        return response.toString();
    }

    public static HashMap<String, String> getControlTableAttributes(){
        HashMap<String, String> attributes = new HashMap<>();
        attributes.put("ITEM_ID", "");
        attributes.put("PRODUCT_NAME", "");
        attributes.put("PART_NUMBER", "");
        attributes.put("SHORT_DESCRIPTION", "");
        attributes.put("BRAND_ID", "");
        attributes.put("CATEGORY_ID", "");
        attributes.put("WEIGHT", "");
        attributes.put("UNIT_OF_MEASUREMENT", "");
        attributes.put("AVAILABLE", "");
        attributes.put("COST", "");
        return attributes;
    }

    public static String bodyValidation(String value){
        if(value == null){
            throw new RuntimeException("Error in body request.");
        }
        return value;
    }

    public static HttpHeaders getHeaders(String accessToken){
        HttpHeaders headers = new HttpHeaders();
        headers.add("Content-Type", "application/json");
        headers.add("Authorization", "Bearer " + accessToken);
        return headers;
    }

    public static HashMap<String, String> getMarginCategoriesBodyRelation(){
        HashMap<String, String> attributes = new HashMap<>();
        attributes.put("PRICE_LIST", "priceListName");
        attributes.put("MARKETPLACE", "marketplace");
        return attributes;
    }

    public static validateDataAreaId(){

    }
}
