package com.vegusa.middleware.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.middleware.entity.Company;
import com.vegusa.middleware.entity.SyncImage;
import com.vegusa.middleware.repository.*;
import com.vegusa.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.middleware.entity.ItemImages;
import com.vegusa.middleware.utils.MWUtils;
import jakarta.persistence.EntityManager;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.core.env.Environment;
import org.springframework.http.HttpHeaders;
import org.springframework.stereotype.Service;
import org.springframework.web.reactive.function.client.WebClient;
import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;

@Service
public class ImageSyncService {
    private final ItemImagesRepository itemImagesRepo;
    private final SyncImageRepository syncImageRepo;
    private final CompanyRepository companyRepo;
    private final EndpointRepository endpointRepo;
    private final AuthTokenRepository authTokenRepo;
    private final WebClient webClient;
    private final Environment env;
    private EncryptDecryptInterface encryptDecryptInterface;
    private String algorithm;

    @Autowired
    public ImageSyncService(ItemImagesRepository itemImagesRepo,
                            SyncImageRepository syncImageRepo,
                            CompanyRepository companyRepo,
                            EndpointRepository endpointRepo,
                            AuthTokenRepository authTokenRepo,
                            EntityManager entityManager,
                            WebClient webClient,
                            Environment env){
        this.itemImagesRepo = itemImagesRepo;
        this.syncImageRepo = syncImageRepo;
        this.companyRepo = companyRepo;
        this.endpointRepo = endpointRepo;
        this.authTokenRepo = authTokenRepo;
        this.webClient = webClient;
        this.env = env;
    }
    
    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface, String algorithm) {
        this.encryptDecryptInterface = encryptDecryptInterface;
        this.algorithm = algorithm;
    }

    public String getAccessToken() throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException {
        return MWUtils.getDecryptedAccessToken(authTokenRepo, encryptDecryptInterface, env, algorithm);
    }

    public String getMerchantId(String accessToken) throws RuntimeException, JsonProcessingException {
        String url = endpointRepo.getEndpointUrl("GET_APP_INFORMATION", env.getProperty("integration.company.name"));
        String appInfo = MWUtils.getAppInfo(webClient, url, accessToken);
        return MWUtils.getJsonNodeResponse(appInfo, "MerchantId");
    }

    public Company getCompany(String dataAreaId) throws RuntimeException {
        return companyRepo.getCompany(dataAreaId);
    }

    public String processImagesUpload(int maxNumOfItemsPerCall, String authToken, String merchantId, String dataAreaId, Company company) throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException {
        JSONObject response = new JSONObject();
        String url = endpointRepo.getEndpointUrl("UPLOAD_PICTURE_TO_PRODUCT_BY_URL", env.getProperty("integration.company.name"))
                .replace("{{merchant_id}}", merchantId) .replace("{{product-pictures-set-id}}", "default");
        ItemImages[] itemImages = itemImagesRepo.getItemImages(dataAreaId);
        String auxPreviousItem = itemImages[0].getMvItemId();
        JSONArray request =  new JSONArray();
        ArrayList<String> images = new ArrayList<>();
        long auxItemProcessed = (long)1, auxRowProcessed = (long)1, streamSize = itemImages.length;
        for(ItemImages imageByProduct: itemImages){
            try {
                if(!imageByProduct.getMvItemId().equals(auxPreviousItem)){
                    insertItemImagesNode(request, images, auxPreviousItem);
                    if(auxItemProcessed % maxNumOfItemsPerCall == 0){
                        String uploadResponse = uploadItemImages(authToken, url, request);
                        JSONArray saveInfoResponse = saveSyncImagesInfo(uploadResponse, company);
                        response.accumulate("ok", saveInfoResponse);
                    }
                    auxItemProcessed++;
                }
                images.add(imageByProduct.getImageUrl());
                auxPreviousItem = imageByProduct.getMvItemId();
                if(auxRowProcessed == streamSize){
                    insertItemImagesNode(request, images, imageByProduct.getMvItemId());
                    String uploadResponse = uploadItemImages(authToken, url, request);
                    JSONArray saveInfoResponse = saveSyncImagesInfo(uploadResponse, company);
                    response.accumulate("ok", saveInfoResponse);
                }
            } catch (RuntimeException e) {
                System.err.println("An error occurred processing product images of: " + imageByProduct.getItemId());
                if(e.getMessage().contains("401")){
                    authToken = MWUtils.getDecryptedAccessToken(authTokenRepo, encryptDecryptInterface, env, algorithm,
                            "An error occurred while renewing unauthorized token.");
                }
                response.accumulate("error", "An error occurred processing product images of: " + imageByProduct.getItemId() + " " + e.getMessage());
            }
            auxRowProcessed++;
            System.out.println("Image synchronization ended to " + imageByProduct.getItemId());
        }
        return response.toString();
    }

    private void insertItemImagesNode(JSONArray request, ArrayList<String> images, String productId) throws RuntimeException {
        JSONObject productImages =  new JSONObject();
        productImages.put("productId", productId);
        productImages.put("images", images);
        request.put(productImages);
        images.clear();
    }

    private String uploadItemImages(String accessToken, String url, JSONArray bodyRequest) {
        try {
            HttpHeaders headers = new HttpHeaders();
            headers.add("Content-Type", "application/json");
            headers.add("Authorization", "Bearer " + accessToken);
            return webClient.post()
                    .uri(url)
                    .headers(h -> h.addAll(headers))
                    .bodyValue(bodyRequest.toString())
                    .retrieve()
                    .bodyToMono(String.class)
                    .block();
        } catch (RuntimeException e){
            System.err.println("An error occurred while uploading item images." + e.getMessage());
            throw new RuntimeException("An error occurred while uploading item images." + e.getMessage());
        } finally {
            bodyRequest.clear();
        }
    }

    private JSONArray saveSyncImagesInfo(String uploadResponse, Company company) {
        try {
            JSONArray response = new JSONArray();
            JSONArray uploadRespArray = new JSONArray(uploadResponse);
            for(int it = 0; it < uploadRespArray.length(); it++){
                try {
                    JSONObject joUpImageInfo = uploadRespArray.optJSONObject(it);
                    JSONArray jaUpImageInfo = joUpImageInfo.getJSONArray("imagesProcess");
                    for(int i = 0; i < jaUpImageInfo.length(); i++){
                        JSONObject imageInfoResponse = new JSONObject();
                        JSONObject upImageInfo = jaUpImageInfo.optJSONObject(i);
                        ObjectMapper objMapUpImageInfo = new ObjectMapper();
                        SyncImage syncImage = objMapUpImageInfo
                                .readValue(upImageInfo.toString(), SyncImage.class);
                        syncImage.setCompany(company);
                        syncImageRepo.save(syncImage);
                        imageInfoResponse.put(syncImage.getProductId(), syncImage.getUrl());
                        response.put(imageInfoResponse);
                    }
                } catch (RuntimeException e) {
                    System.err.println("An error occurred while saving uploaded image info to: " + uploadRespArray.optJSONObject(it));
                }
            }
            return response;
        } catch (RuntimeException | JsonProcessingException e) {
            System.err.println("Error occurred while saving uploaded images info to Middleware data base: " + "\r" + e.getMessage());
            throw new RuntimeException("An error occurred while saving uploaded images info to Middleware data base: " + e.getMessage());
        }
    }

}
