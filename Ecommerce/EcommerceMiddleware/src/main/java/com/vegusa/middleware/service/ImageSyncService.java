package com.vegusa.middleware.service;

import com.azure.storage.blob.BlobContainerClientBuilder;
import com.azure.storage.blob.BlobContainerClient;
import com.azure.storage.blob.specialized.BlockBlobClient;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.repository.*;
import com.vegusa.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.middleware.utils.MWUtils;
import com.vegusa.oauth2_0.service.AuthService;
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
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.net.URI;
import java.net.URISyntaxException;
import java.net.URL;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.HashMap;

@Service
public class ImageSyncService {
    private final ProductImagesViewRepository productImagesViewRepo;
    private final SyncItemRepository syncItemRepo;
    private final SyncImageRepository syncImageRepo;
    private final CompanyRepository companyRepo;
    private final EndpointRepository endpointRepo;
    private final AuthTokenRepository authTokenRepo;
    private final AuthService authService;
    private final WebClient webClient;
    private final Environment env;
    private EncryptDecryptInterface encryptDecryptInterface;
    private String algorithm;

    @Autowired
    public ImageSyncService(ProductImagesViewRepository productImagesViewRepo,
                            SyncItemRepository syncItemRepo,
                            SyncImageRepository syncImageRepo,
                            CompanyRepository companyRepo,
                            EndpointRepository endpointRepo,
                            AuthTokenRepository authTokenRepo,
                            AuthService authService,
                            EntityManager entityManager,
                            WebClient webClient,
                            Environment env){
        this.productImagesViewRepo = productImagesViewRepo;
        this.syncItemRepo = syncItemRepo;
        this.syncImageRepo = syncImageRepo;
        this.companyRepo = companyRepo;
        this.endpointRepo = endpointRepo;
        this.authTokenRepo = authTokenRepo;
        this.authService = authService;
        this.webClient = webClient;
        this.env = env;
    }
    
    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface, String algorithm) {
        this.encryptDecryptInterface = encryptDecryptInterface;
        this.algorithm = algorithm;
    }

    public String getAccessToken() throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException {
        AuthToken tokenInfo =  authTokenRepo.getAuthToken(env.getProperty("integration.company.name"));
        return MWUtils.getDecryptedAccessToken(tokenInfo, encryptDecryptInterface, algorithm);
    }

    public String getMerchantId(String accessToken) throws RuntimeException, JsonProcessingException {
        String url = endpointRepo.getEndpointUrl("GET_APP_INFORMATION", env.getProperty("integration.company.name"));
        String appInfo = MWUtils.getAppInfo(webClient, url, accessToken);
        return MWUtils.getJsonNodeResponse(appInfo, "MerchantId");
    }

    public Company getCompany(String dataAreaId) throws RuntimeException {
        return companyRepo.getCompany(dataAreaId);
    }

    public String processImagesUpload(int maxNumOfItemsPerCall, String authToken, String merchantId, String dataAreaId, Company company, String albumId) throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException {
        JSONObject response = new JSONObject();
        String url = endpointRepo.getEndpointUrl("UPLOAD_PICTURE_TO_PRODUCT_BY_URL", env.getProperty("integration.company.name"))
                .replace("{{merchant_id}}", merchantId) .replace("{{product-pictures-set-id}}", albumId);
        ProductImagesView[] productImages = productImagesViewRepo.getProductImagesView(dataAreaId);
        String auxPreviousItem = productImages[0].getSyncItemResponseId();
        JSONArray request =  new JSONArray();
        ArrayList<String> images = new ArrayList<>();
        long auxItemProcessed = (long)1, auxRowProcessed = (long)1, streamSize = productImages.length;
        for(ProductImagesView imageByProduct: productImages){
            try {
                if(!imageByProduct.getSyncItemResponseId().equals(auxPreviousItem)){
                    boolean imagesIsEmpty = images.isEmpty();
                    insertItemImagesNode(request, images, auxPreviousItem, imagesIsEmpty);
                    if(auxItemProcessed % maxNumOfItemsPerCall == 0 && !imagesIsEmpty){
                        String uploadResponse = uploadItemImages(authToken, url, request);
                        JSONArray saveInfoResponse = saveSyncImagesInfo(uploadResponse, company);
                        response.accumulate("ok", saveInfoResponse);
                    }
                    if(!imagesIsEmpty){
                        auxItemProcessed++;
                    }
                }
                insertImage(images, imageByProduct, albumId);
                auxPreviousItem = imageByProduct.getSyncItemResponseId();
                if(auxRowProcessed == streamSize){
                    boolean imagesIsEmpty = images.isEmpty();
                    insertItemImagesNode(request, images, auxPreviousItem, imagesIsEmpty);
                    if(!request.isEmpty()){
                        String uploadResponse = uploadItemImages(authToken, url, request);
                        JSONArray saveInfoResponse = saveSyncImagesInfo(uploadResponse, company);
                        response.accumulate("ok", saveInfoResponse);
                    }
                }
            } catch (RuntimeException e) {
                System.err.println("An error occurred processing product images: " + e.getMessage());
                if(e.getMessage().contains("401") || e.getMessage().contains("404")){
//                    authToken = MWUtils.getDecryptedAccessToken(authService.getAuthToken(), encryptDecryptInterface, algorithm,
//                            "An error occurred while renewing unauthorized token.");
                    System.out.println("error: " + authToken);
                }
                response.accumulate("error", "An error occurred processing product images: " + e.getMessage());
            }
            auxRowProcessed++;
            System.out.println(imageByProduct.getItemId() + "/" + imageByProduct.getBlobName() + " processed. ");
        }
        if(response.isEmpty()){
            response.accumulate("ok", "The process ended, there are no new images to synchronize.");
        }
        return response.toString();
    }

    private void insertImage(ArrayList<String> images, ProductImagesView imageByProduct, String albumId) throws RuntimeException{
        SyncImage syncImage = syncImageRepo.getSyncImage(imageByProduct.getBlobName(), albumId);
        if(syncImage == null) {
            images.add(imageByProduct.getImageUrl());
        }
    }

    private void insertItemImagesNode(JSONArray request, ArrayList<String> images, String productId, boolean imagesIsEmpty) throws RuntimeException {
        if(!imagesIsEmpty){
            JSONObject productImages =  new JSONObject();
            productImages.put("productId", productId);
            productImages.put("images", images);
            request.put(productImages);
        }
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
            throw new RuntimeException("An error occurred while uploading item images to " + bodyRequest  + " " + e.getMessage());
        } finally {
            bodyRequest.clear();
        }
    }

    private JSONArray saveSyncImagesInfo(String uploadResponse, Company company) {
        try {
            JSONArray response = new JSONArray();

            //test
            JSONObject jsonObject = new JSONObject();
            jsonObject.put("uploadResponse", uploadResponse);
            response.put(jsonObject);

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
            throw new RuntimeException("An error occurred while saving uploaded images info to Middleware data base " +  e.getMessage());
        }
    }

}
