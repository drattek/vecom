package com.vegusa.veg_mv_integration_midd.msb.controller;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.vegusa.veg_mv_integration_midd.msb.service.*;
import com.vegusa.veg_mv_integration_midd.veg_middleware.utils.MiddUtils;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;
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
    private final ProductControlTableService controlTable;
    private final InterfaceInformationService interfaceInfo;
    private final MSBWebScraperService webScraperService;

    @Autowired
    public MSBController(MSBProductSyncService msbProductSyncService,
                         MSBImagesSyncService msbImagesSyncService,
                         ProductControlTableService controlTable,
                         InterfaceInformationService interfaceInfo,
                         MSBWebScraperService webScraperService){
        this.msbProductSyncService = msbProductSyncService;
        this.msbImagesSyncService = msbImagesSyncService;
        this.controlTable = controlTable;
        this.interfaceInfo = interfaceInfo;
        this.webScraperService = webScraperService;
    }

    @PostMapping(value = "/synchronize-products")
    public String uploadProducts() {
        try {
            msbProductSyncService.setEncryptDecryptInterface(MiddUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            return msbProductSyncService.processAndUploadProducts();
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | JsonProcessingException e){
            System.err.println("An error occurred while synchronizing the products.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/synchronize-images")
    public String uploadImages() {
        try {
            msbImagesSyncService.setEncryptDecryptInterface(MiddUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            return msbImagesSyncService.processAndUploadProductsImages(50);
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                 NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | JsonProcessingException e) {
            System.err.println("An error occurred while uploading the images.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/update-products-control-table")
    public String updateProductsControlTable(@RequestBody HashMap<String, String> interfaceDS){
        try {
            return controlTable.processAndSaveInterfaceInfo(interfaceDS.get("interfaceId"), interfaceDS.get("dataAreaId"));
        } catch (RuntimeException e){
            System.err.println("An error occurred while updating product control table.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/update-interface-information")
    public String updateInterfaceInfo(@RequestBody HashMap<String, String> interfaceDS){
        try {
            return switch (interfaceDS.get("interfaceId")) {
                case "DYN" ->
                        interfaceInfo.updateDYNInterfaceInfo(interfaceDS.get("dataAreaId"), interfaceDS.get("interfaceId"));
                case "UCA", "TVH" ->
                        interfaceInfo.updateInterfaceInfo(interfaceDS.get("dataAreaId"), interfaceDS.get("interfaceId"));
                default -> throw new RuntimeException("The interfaceId sent doesn't exist.");
            };
        } catch (RuntimeException e){
            System.err.println("An error occurred while updating interface information.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/add-tvh-additional-info")
    public String uploadTVHAdditionalInfo() {
        try{
            msbProductSyncService.setEncryptDecryptInterface(MiddUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            return msbProductSyncService.processTVHAdditionalInfo();
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                 NoSuchAlgorithmException | BadPaddingException | InvalidKeyException e){
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/tvh-image-scraper")
    public String tvhWebScraperImages(@RequestBody HashMap<String, String> userCredentials) {
        try {
            String bodyRequest = webScraperService.getJsonRequest(userCredentials.get("userEmail"), userCredentials.get("userPass"));
            return webScraperService.getTVHScrapedImages(bodyRequest);
        } catch (RuntimeException e) {
            System.err.println("An error occurred while scraping the images.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/tvh-products-info-scraper")
    public String tvhWebScraperProductsInfo(@RequestBody HashMap<String, String> userCredentials) {
        try {
            String bodyRequest = webScraperService.getJsonRequest(userCredentials.get("userEmail"), userCredentials.get("userPass"));
            return webScraperService.getTVHProductsInfo(bodyRequest);
        } catch (RuntimeException e) {
            System.err.println("An error occurred while scraping products info.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/uca-search-additional-info")
    public String getUCAAdditionalProductsInfo() {
        try {
            String bodyRequest = webScraperService.getJsonRequest("","");
            return webScraperService.getUCAProductsInfo(bodyRequest);
        } catch (RuntimeException e) {
            System.err.println("An error occurred while searching products info.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }
}
