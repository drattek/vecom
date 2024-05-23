package com.vegusa.veg_mv_integration_midd.msb.controller;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.vegusa.veg_mv_integration_midd.msb.service.MSBImageWebScraperService;
import com.vegusa.veg_mv_integration_midd.msb.service.MSBImagesSyncService;
import com.vegusa.veg_mv_integration_midd.msb.service.MSBProductSyncService;
import com.vegusa.veg_mv_integration_midd.veg_middleware.utils.MiddUtils;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.util.MultiValueMap;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.reactive.function.client.WebClientResponseException;

import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.text.ParseException;
import java.util.HashMap;

@RestController
@RequestMapping("msb-ecomm-integration")
public class MSBController {
    private final MSBProductSyncService msbProductSyncService;
    private final MSBImageWebScraperService imageWebScraperService;
    private final MSBImagesSyncService imagesSyncService;

    @Autowired
    public MSBController(MSBProductSyncService msbProductSyncService,
                         MSBImageWebScraperService imageWebScraperService,
                         MSBImagesSyncService imagesSyncService){
        this.msbProductSyncService = msbProductSyncService;
        this.imageWebScraperService = imageWebScraperService;
        this.imagesSyncService = imagesSyncService;
    }

    @PostMapping(value = "/synchronize-products")
    public String uploadProducts() {
        try {
            msbProductSyncService.setEncryptDecryptInterface(MiddUtils.getEncryptDecryptInterface());
            return msbProductSyncService.processProducts();
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | JsonProcessingException e){
            System.err.println("An error occurred while synchronizing the products: ");
            e.printStackTrace();
            return "An error occurred while synchronizing the products: " + "\r" + e.getMessage();
        }
    }

    @PostMapping(value="/image-scraper")
    public String tvhWebScraper(@RequestBody HashMap<String, String> userCredentials){
        try {
            String bodyRequest = imageWebScraperService.getJSONRequest(userCredentials.get("userEmail"), userCredentials.get("userPass"));
            return  imageWebScraperService.getTVHScrapedImages(bodyRequest);
        } catch (WebClientResponseException e){
            return  e.getMessage();
        }
    }

    @PostMapping(value="/synchronize-images")
    public String uploadImages() {
        try {
            String requestBody;
            String uploadResponse;
            imagesSyncService.setEncryptDecryptInterface(MiddUtils.getEncryptDecryptInterface());
            requestBody = imagesSyncService.getJSONToSyncProductsImages();
            uploadResponse = imagesSyncService.uploadProductImages(requestBody);
            return imagesSyncService.updateMiddlewareSynchronizedImages(uploadResponse);
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                 NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | JsonProcessingException e) {
            return e.getMessage();
        }
    }









}
