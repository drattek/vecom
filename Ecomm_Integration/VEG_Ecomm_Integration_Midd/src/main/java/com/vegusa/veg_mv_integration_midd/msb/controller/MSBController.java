package com.vegusa.veg_mv_integration_midd.msb.controller;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.vegusa.veg_mv_integration_midd.msb.service.MSBImageWebScraperService;
import com.vegusa.veg_mv_integration_midd.msb.service.MSBImagesSyncService;
import com.vegusa.veg_mv_integration_midd.msb.service.MSBProductSyncService;
import com.vegusa.veg_mv_integration_midd.veg_middleware.utils.MiddUtils;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.*;

import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
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
            System.err.println("An error occurred while synchronizing the products.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            return "An error occurred while synchronizing the products: " + "\r" + e.getMessage();
        }
    }

    @PostMapping(value="/image-scraper")
    public String tvhWebScraper(@RequestBody HashMap<String, String> userCredentials) {
        try {
            String bodyRequest = imageWebScraperService.getJSONRequest(userCredentials.get("userEmail"), userCredentials.get("userPass"));
            return imageWebScraperService.getTVHScrapedImages(bodyRequest);
        } catch (RuntimeException e){
            System.err.println("An error occurred while scraping the images.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            return "An error occurred while scraping the images: " + "\r" + e.getMessage();
        }
    }

    @PostMapping(value="/synchronize-images")
    public String uploadImages() {
        try {
            String requestBody;
            JsonNode uploadResponse;
            imagesSyncService.setEncryptDecryptInterface(MiddUtils.getEncryptDecryptInterface());
            requestBody = imagesSyncService.getJSONToSyncProductsImages();
            uploadResponse = MiddUtils.validateResponse("Error while uploading images to ecommerce: ", imagesSyncService.uploadProductImages(requestBody));
            return imagesSyncService.updateMiddlewareSynchronizedImages(uploadResponse);
        } catch (RuntimeException | JsonProcessingException e) {
            System.err.println("An error occurred while uploading the images.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            return "An error occurred while uploading the images: " + "\r" + e.getMessage();
        }
    }

}
