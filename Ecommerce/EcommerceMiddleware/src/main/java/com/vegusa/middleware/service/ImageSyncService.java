package com.vegusa.middleware.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.middleware.repository.TokenInfoRepository;
import com.vegusa.middleware.repository.VegEcomSynchronizedImagesRepository;
import com.vegusa.middleware.repository.VegEcomvIntegrationEndptsRepository;
import com.vegusa.middleware.repository.VwVegImagesByProductRepository;
import com.vegusa.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.middleware.entity.VegEcomSynchronizedImages;
import com.vegusa.middleware.entity.VwVegImagesByProduct;
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
    private final VwVegImagesByProductRepository vwVegImagesByProductRepository;
    private final VegEcomSynchronizedImagesRepository vegEcommSynchronizedImageRepository;
    private final TokenInfoRepository tokenInfoRepository;
    private final VegEcomvIntegrationEndptsRepository endptsRepository;
    private final EntityManager entityManager;
    private final WebClient webClient;
    private final Environment env;
    private EncryptDecryptInterface encryptDecryptInterface;
    private String algorithm;

    @Autowired
    public ImageSyncService(VwVegImagesByProductRepository vwVegImagesByProductRepository,
                            VegEcomSynchronizedImagesRepository vegEcommSynchronizedImageRepository,
                            TokenInfoRepository tokenInfoRepository,
                            VegEcomvIntegrationEndptsRepository endptsRepository,
                            EntityManager entityManager,
                            WebClient webClient,
                            Environment env){
        this.vwVegImagesByProductRepository = vwVegImagesByProductRepository;
        this.vegEcommSynchronizedImageRepository = vegEcommSynchronizedImageRepository;
        this.tokenInfoRepository = tokenInfoRepository;
        this.endptsRepository = endptsRepository;
        this.entityManager = entityManager;
        this.webClient = webClient;
        this.env = env;
    }

    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface, String algorithm) {
        this.encryptDecryptInterface = encryptDecryptInterface;
        this.algorithm = algorithm;
    }

    @Transactional(readOnly = false)
    public String processAndUploadProductsImages(int maxNumOfProductsPerCall) throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException {
        JSONObject response = new JSONObject();
        AtomicReference<String> accessToken = new AtomicReference<>(MWUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface, env, algorithm));
        JsonNode jsonNodeAppInfo = MWUtils
                .validateResponse("An error occurred while obtaining App Information: ",
                        MWUtils.getAppInfo(webClient, endptsRepository.getIntegrationEndPoint("GET_APP_INFORMATION", "MULTIVENDE"), accessToken.get()));
        String url = endptsRepository.getIntegrationEndPoint("UPLOAD_PICTURE_TO_PRODUCT_BY_URL", "MULTIVENDE")
                .replace("{{merchant_id}}", jsonNodeAppInfo.get("MerchantId").asText())
                .replace("{{product-pictures-set-id}}", "default");
        Supplier<Stream<VwVegImagesByProduct>> vwVegImagesByProductIdStream = () -> vwVegImagesByProductRepository.getSynchronizedProductsWithImages();
        long auxStreamSize = vwVegImagesByProductIdStream.get().count();
        AtomicLong auxProcessedProduct = new AtomicLong(1);
        AtomicLong auxProcessedRow = new AtomicLong(1);
        AtomicReference<String> auxPrevProductId = new AtomicReference<>(vwVegImagesByProductRepository.getFstSyncProductIDWithImage());
        JSONArray request =  new JSONArray();
        ArrayList<String> images = new ArrayList<>();
        vwVegImagesByProductIdStream.get().forEach(imageByProduct -> {
            try {
                if(!imageByProduct.getIdMv().equals(auxPrevProductId.get())){
                    insertProductImagesNode(request, images, auxPrevProductId.get());
                    if(auxProcessedProduct.get() % maxNumOfProductsPerCall == 0){
                        JsonNode auxResp = MWUtils
                                .validateResponse("An error occurred while uploading the images. ",
                                        uploadProductImages(accessToken.get(), url, request));
                        JSONArray auxRespMiddleware = updateMiddlewareSynchronizedImages(auxResp.toString());
                        response.accumulate("ok", auxRespMiddleware);
                    }
                    auxProcessedProduct.getAndIncrement();
                }
                images.add(imageByProduct.getImageUrl());
                auxPrevProductId.set(imageByProduct.getIdMv());
                if(auxProcessedRow.get() == auxStreamSize){
                    insertProductImagesNode(request, images, imageByProduct.getIdMv());
                    JsonNode auxResp = MWUtils
                            .validateResponse("An error occurred while uploading the images. ",
                                    uploadProductImages(accessToken.get(), url, request));
                    JSONArray auxRespMiddleware = updateMiddlewareSynchronizedImages(auxResp.toString());
                    response.accumulate("ok", auxRespMiddleware);
                }
            } catch (RuntimeException | JsonProcessingException e) {
                System.err.println("An error occurred processing product images of: " + imageByProduct.getInternalCode());
                e.printStackTrace();
                if(e.getMessage().contains("401")){
                    accessToken.set(MWUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface, env, algorithm,
                            "Access token was not found in processProductsImages method."));
                }
                response.accumulate("error", e.getMessage());
            }
            auxProcessedRow.getAndIncrement();
            entityManager.detach(imageByProduct);
        });
        return response.toString();
    }

    private void insertProductImagesNode(JSONArray request,  ArrayList<String> images, String productId) throws RuntimeException {
        JSONObject productImages =  new JSONObject();
        productImages.put("productId", productId);
        productImages.put("images", images);
        request.put(productImages);
        images.clear();
    }

    public String uploadProductImages(String accessToken, String url, JSONArray bodyRequest) {
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
            return MWUtils.getSimpleJSONResponse("error", e.getMessage());
        } finally {
            bodyRequest.clear();
        }
    }

    public JSONArray updateMiddlewareSynchronizedImages(String uploadResponse) {
        try {
            JSONArray response = new JSONArray();
            MWUtils.validateResponse("Error occurred while uploading images to ecommerce: ", uploadResponse);
            JSONArray uploadRespArray = new JSONArray(uploadResponse);
            for(int it = 0; it < uploadRespArray.length(); it++){
                try {
                    JSONObject joUpImageInfo = uploadRespArray.optJSONObject(it);
                    JSONArray jaUpImageInfo = joUpImageInfo.getJSONArray("imagesProcess");
                    for(int i = 0; i < jaUpImageInfo.length(); i++){
                        JSONObject upImageInfo = jaUpImageInfo.optJSONObject(i);
                        ObjectMapper objMapUpImageInfo = new ObjectMapper();
                        VegEcomSynchronizedImages vegEcommSynchronizedImage = objMapUpImageInfo
                                .readValue(upImageInfo.toString(), VegEcomSynchronizedImages.class);
                        vegEcommSynchronizedImageRepository.save(vegEcommSynchronizedImage);
                        response.put(new JSONObject(vegEcommSynchronizedImage.getProductId(), vegEcommSynchronizedImage.getUrl()));
                    }
                } catch (RuntimeException e) {
                    System.err.println("An error occurred while saving uploaded image info to: " + uploadRespArray.optJSONObject(it));
                    e.printStackTrace();
                }
            }
            return response;
        } catch (RuntimeException | JsonProcessingException e) {
            System.err.println("Error occurred while saving uploaded images info to Middleware data base: " + "\r" + e.getMessage());
            throw new RuntimeException(e.getMessage());
        }
    }

}
