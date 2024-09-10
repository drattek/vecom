package com.vegusa.middleware.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.middleware.repository.AuthTokenRepository;
import com.vegusa.middleware.repository.SyncImageRepository;
import com.vegusa.middleware.repository.EndpointRepository;
import com.vegusa.middleware.repository.ItemImagesRepository;
import com.vegusa.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.middleware.entity.SyncImage;
import com.vegusa.middleware.entity.ItemImages;
import com.vegusa.middleware.utils.MWUtils;
import jakarta.persistence.EntityManager;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.core.env.Environment;
import org.springframework.http.HttpHeaders;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.reactive.function.client.WebClient;
import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.atomic.AtomicReference;
import java.util.function.Supplier;
import java.util.stream.Stream;

@Service
public class ImageSyncService {
    private final ItemImagesRepository itemImagesRepo;
    private final SyncImageRepository syncImageRepo;
    private final EndpointRepository endpointRepo;
    private final AuthTokenRepository authTokenRepo;
    private final EntityManager entityManager;
    private final WebClient webClient;
    private final Environment env;
    private EncryptDecryptInterface encryptDecryptInterface;
    private String algorithm;

    @Autowired
    public ImageSyncService(ItemImagesRepository itemImagesRepo,
                            SyncImageRepository syncImageRepo,
                            EndpointRepository endpointRepo,
                            AuthTokenRepository authTokenRepo,
                            EntityManager entityManager,
                            WebClient webClient,
                            Environment env){
        this.itemImagesRepo = itemImagesRepo;
        this.syncImageRepo = syncImageRepo;
        this.endpointRepo = endpointRepo;
        this.authTokenRepo = authTokenRepo;
        this.entityManager = entityManager;
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

    @Transactional(readOnly = false)
    public String processImagesSync(AtomicReference<String> authToken, String merchantId, int maxNumOfItemsPerCall) throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException {
        JSONObject response = new JSONObject();
        String url = endpointRepo.getEndpointUrl("UPLOAD_PICTURE_TO_PRODUCT_BY_URL", env.getProperty("integration.company.name"))
                .replace("{{merchant_id}}", merchantId) .replace("{{product-pictures-set-id}}", "default");
        Supplier<Stream<ItemImages>> itemImages = itemImagesRepo::getItemImages;
        AtomicReference<String> auxPreviousItem = new AtomicReference<>(itemImagesRepo.getFstItemImageId());
        JSONArray request =  new JSONArray();
        ArrayList<String> images = new ArrayList<>();
        AtomicLong auxItemProcessed = new AtomicLong(1), auxRowProcessed = new AtomicLong(1);
        long streamSize = itemImages.get().count();
        itemImages.get().forEach(imageByProduct -> {
            try {
                if(!imageByProduct.getIdMv().equals(auxPreviousItem.get())){
                    insertItemImagesNode(request, images, auxPreviousItem.get());
                    if(auxItemProcessed.get() % maxNumOfItemsPerCall == 0){
                        String uploadResponse = uploadItemImages(authToken.get(), url, request);
                        JSONArray saveInfoResponse = saveSyncImagesInfo(uploadResponse);
                        response.accumulate("ok", saveInfoResponse);
                    }
                    auxItemProcessed.getAndIncrement();
                }
                images.add(imageByProduct.getImageUrl());
                auxPreviousItem.set(imageByProduct.getIdMv());
                if(auxRowProcessed.get() == streamSize){
                    insertItemImagesNode(request, images, imageByProduct.getIdMv());
                    String uploadResponse = uploadItemImages(authToken.get(), url, request);
                    JSONArray saveInfoResponse = saveSyncImagesInfo(uploadResponse);
                    response.accumulate("ok", saveInfoResponse);
                }
            } catch (RuntimeException e) {
                System.err.println("An error occurred processing product images of: " + imageByProduct.getInternalCode());
                if(e.getMessage().contains("401")){
                    authToken.set(MWUtils.getDecryptedAccessToken(authTokenRepo, encryptDecryptInterface, env, algorithm,
                            "Authorization token was not found in Middleware data base."));
                }
                response.accumulate("error", e.getMessage());
            }
            auxRowProcessed.getAndIncrement();
            entityManager.detach(imageByProduct);
        });
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

    private JSONArray saveSyncImagesInfo(String uploadResponse) {
        try {
            JSONArray response = new JSONArray();
            JSONArray uploadRespArray = new JSONArray(uploadResponse);
            for(int it = 0; it < uploadRespArray.length(); it++){
                try {
                    JSONObject joUpImageInfo = uploadRespArray.optJSONObject(it);
                    JSONArray jaUpImageInfo = joUpImageInfo.getJSONArray("imagesProcess");
                    for(int i = 0; i < jaUpImageInfo.length(); i++){
                        JSONObject upImageInfo = jaUpImageInfo.optJSONObject(i);
                        ObjectMapper objMapUpImageInfo = new ObjectMapper();
                        SyncImage vegEcommSynchronizedImage = objMapUpImageInfo
                                .readValue(upImageInfo.toString(), SyncImage.class);
                        syncImageRepo.save(vegEcommSynchronizedImage);
                        response.put(new JSONObject(vegEcommSynchronizedImage.getProductId(), vegEcommSynchronizedImage.getUrl()));
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
