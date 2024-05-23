package com.vegusa.veg_mv_integration_midd.msb.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonMappingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.veg_mv_integration_midd.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegEcommScrapedImage;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegEcommSynchronizedImage;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegMvSynchronizedProduct;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.*;
import com.vegusa.veg_mv_integration_midd.veg_middleware.utils.MiddUtils;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpHeaders;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.util.LinkedMultiValueMap;
import org.springframework.util.MultiValueMap;
import org.springframework.web.reactive.function.BodyInserters;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.reactive.function.client.WebClientResponseException;

import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import java.lang.reflect.InvocationTargetException;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.stream.Stream;

@Service
public class MSBImagesSyncService {
    private VegMvSynchronizedProductRepository vegMvSynchronizedProductRepository;
    private VegEcommScrapedImageRepository vegEcommScrapedImageRepository;
    private VegEcommSynchronizedImageRepository vegEcommSynchronizedImageRepository;
    private final TokenInfoRepository tokenInfoRepository;
    private final VegMvIntegrationEndptsRepository endptsRepository;
    private final WebClient webClient;
    private EncryptDecryptInterface encryptDecryptInterface;

    @Autowired
    public MSBImagesSyncService(VegMvSynchronizedProductRepository vegMvSynchronizedProductRepository,
                                VegEcommScrapedImageRepository vegEcommScrapedImageRepository,
                                VegEcommSynchronizedImageRepository vegEcommSynchronizedImageRepository,
                                TokenInfoRepository tokenInfoRepository,
                                VegMvIntegrationEndptsRepository endptsRepository,
                                WebClient webClient){
        this.vegMvSynchronizedProductRepository = vegMvSynchronizedProductRepository;
        this.vegEcommScrapedImageRepository = vegEcommScrapedImageRepository;
        this.vegEcommSynchronizedImageRepository = vegEcommSynchronizedImageRepository;
        this.tokenInfoRepository = tokenInfoRepository;
        this.endptsRepository = endptsRepository;
        this.webClient = webClient;
    }

    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface) {
        this.encryptDecryptInterface = encryptDecryptInterface;
    }

    @Transactional(readOnly = false)
    public String getJSONToSyncProductsImages() throws RuntimeException {
        Stream<VegMvSynchronizedProduct> productsStream = vegMvSynchronizedProductRepository.getSynchronizedProducts();
        JSONArray request =  new JSONArray();
        for (Iterator<VegMvSynchronizedProduct> it = productsStream.iterator(); it.hasNext(); ) {
            VegMvSynchronizedProduct productStream = it.next();
            ArrayList<String> images = new ArrayList<>();
            VegEcommScrapedImage[] vegEcommScrapedImage = vegEcommScrapedImageRepository.getImagesOfProduct(productStream.getInternalCode());
            for(int i = 0; i < vegEcommScrapedImage.length; i++){
                images.add(vegEcommScrapedImage[i].getImageUrl());
            }
            if(images.size() != 0){
                JSONObject productImages =  new JSONObject();
                productImages.put("productId", productStream.getIdMvd());
                productImages.put("images", images);
                request.put(productImages);
            }
        }
        return request.toString();
    }

    public String uploadProductImages(String bodyRequest) throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException {
        String accessToken = MiddUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface);
        ObjectMapper objMapAppInfo = new ObjectMapper();
        JsonNode jsonNodeAppInfo = objMapAppInfo.
                readTree(MiddUtils.getAppInfo(webClient, endptsRepository.getEndPointMuitiVende("GET_APP_INFORMATION"), accessToken));
        if(jsonNodeAppInfo.has("error")) {
            System.err.println("An error occurred while obtaining App Information.");
            throw new RuntimeException("An error occurred while obtaining App Information." + jsonNodeAppInfo.get("error").asText());
        } else {
            HttpHeaders headers = new HttpHeaders();
            headers.add("Content-Type", "application/json");
            headers.add("Authorization", "Bearer " + accessToken);
            String url = endptsRepository.getEndPointMuitiVende("UPLOAD_PICTURE_TO_PRODUCT_BY_URL")
                    .replace("{{merchant_id}}", jsonNodeAppInfo.get("MerchantId").asText())
                    .replace("{{product-pictures-set-id}}", "default");
            return webClient.post()
                    .uri(url)
                    .headers(h -> h.addAll(headers))
                    .bodyValue(bodyRequest)
                    .retrieve()
                    .bodyToMono(String.class)
                    .block();
        }
    }

    public String updateMiddlewareSynchronizedImages(String uploadResponse) throws JsonProcessingException, RuntimeException {
        JSONObject response = new JSONObject();
        JSONArray uploadRespArray = new JSONArray(uploadResponse);
        for(int it = 0; it < uploadRespArray.length(); it++){
            JSONObject joUpImageInfo = uploadRespArray.optJSONObject(it);
            JSONArray jaUpImageInfo = joUpImageInfo.getJSONArray("imagesProcess");
            for(int i = 0; i < jaUpImageInfo.length(); i++){
                JSONObject upImageInfo = jaUpImageInfo.optJSONObject(i);
                ObjectMapper objMapUpImageInfo = new ObjectMapper();
                VegEcommSynchronizedImage vegEcommSynchronizedImage = objMapUpImageInfo
                        .readValue(upImageInfo.toString(), VegEcommSynchronizedImage.class);
                vegEcommSynchronizedImageRepository.save(vegEcommSynchronizedImage);
            }
        }
        response.put("ok", "Image synchronization completed successfully.");
        response.put("message", uploadRespArray);
        return response.toString();
    }






}
