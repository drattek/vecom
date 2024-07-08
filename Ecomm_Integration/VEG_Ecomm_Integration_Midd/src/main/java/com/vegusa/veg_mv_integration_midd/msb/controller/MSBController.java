package com.vegusa.veg_mv_integration_midd.msb.controller;

import com.fasterxml.jackson.core.JsonProcessingException;
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
@RequestMapping("msb-ecommerce-integration")
public class MSBController {
    private final MSBProductSyncService msbProductSyncService;
    private final MSBImagesSyncService msbImagesSyncService;
    private final MSBImageWebScraperService imageWebScraperService;

    @Autowired
    public MSBController(MSBProductSyncService msbProductSyncService,
                         MSBImagesSyncService msbImagesSyncService,
                         MSBImageWebScraperService imageWebScraperService){
        this.msbProductSyncService = msbProductSyncService;
        this.msbImagesSyncService = msbImagesSyncService;
        this.imageWebScraperService = imageWebScraperService;
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

    @PostMapping(value="/synchronize-images")
    public String uploadImages() {
        try {
            msbImagesSyncService.setEncryptDecryptInterface(MiddUtils.getEncryptDecryptInterface());
            return msbImagesSyncService.processProductsImages(50);
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                 NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | JsonProcessingException e) {
            System.err.println("An error occurred while uploading the images.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            return "An error occurred while uploading the images: " + "\r" + e.getMessage();
        }
    }

    @PostMapping(value="/add-tvh-additional-info")
    public String uploadTVHAdditionalInfo() {
        try{
            msbProductSyncService.setEncryptDecryptInterface(MiddUtils.getEncryptDecryptInterface());
            return msbProductSyncService.processTVHAdditionalInfo();
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException
                | NoSuchAlgorithmException | BadPaddingException | InvalidKeyException e){
            return "";
        }
    }

    @PostMapping(value="/tvh-image-scraper")
    public String tvhWebScraperImages(@RequestBody HashMap<String, String> userCredentials) {
        try {
            String bodyRequest = imageWebScraperService.getJsonRequest(userCredentials.get("userEmail"), userCredentials.get("userPass"));
            return imageWebScraperService.getTVHScrapedImages(bodyRequest);
        } catch (RuntimeException e) {
            System.err.println("An error occurred while scraping the images.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            return "An error occurred while scraping the images: " + "\r" + e.getMessage();
        }
    }

    @PostMapping(value="/tvh-products-info-scraper")
    public String tvhWebScraperProductsInfo(@RequestBody HashMap<String, String> userCredentials) {
        try {
            String bodyRequest = imageWebScraperService.getJsonRequest(userCredentials.get("userEmail"), userCredentials.get("userPass"));
            return imageWebScraperService.getTVHProductsInfo(bodyRequest);
        } catch (RuntimeException e) {
            System.err.println("An error occurred while scraping products info.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            return "An error occurred while scraping the images: " + "\r" + e.getMessage();
        }
    }

    @PostMapping(value="/uca-search-additional-info")
    public String getUCAAdditionalProductsInfo() {
        try {
            String bodyRequest = imageWebScraperService.getJsonRequest("","");
            return imageWebScraperService.getUCAProductsInfo(bodyRequest);
        } catch (RuntimeException e) {
            System.err.println("An error occurred while searching products info.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            return "An error occurred while searching products info: " + "\r" + e.getMessage();
        }
    }

}
