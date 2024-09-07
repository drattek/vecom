package com.vegusa.msb.controller;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.vegusa.middleware.entity.SynchronizedPriceList;
import com.vegusa.middleware.service.*;
import com.vegusa.middleware.utils.MWUtils;
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
import java.util.concurrent.atomic.AtomicReference;

@RestController
@RequestMapping("msb-ecommerce-middleware")
public class MSBController {
    private final ItemSyncService itemSyncService;
    private final ItemControlTableService ctrlTableService;
    private final InterfaceInfoService interfaceInfoService;
    private final ItemInventorySyncService itemInventService;
    private final ItemPriceSyncService itemPriceSyncService;
    private final ImageSyncService imageSyncService;
    private final WebScraperService webScraperService;

    @Autowired
    public MSBController(ItemSyncService itemSyncService,
                         ItemControlTableService ctrlTableService,
                         InterfaceInfoService interfaceInfoService,
                         ItemInventorySyncService itemInventService,
                         ItemPriceSyncService itemPriceSyncService,
                         ImageSyncService imageSyncService,
                         WebScraperService webScraperService){
        this.itemSyncService = itemSyncService;
        this.ctrlTableService = ctrlTableService;
        this.interfaceInfoService = interfaceInfoService;
        this.itemInventService = itemInventService;
        this.itemPriceSyncService = itemPriceSyncService;
        this.imageSyncService = imageSyncService;
        this.webScraperService = webScraperService;
    }

    @PostMapping(value = "/synchronize-products")
    public String uploadProducts(@RequestBody HashMap<String, String> bodyRequest) {
        try {
            itemSyncService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            return itemSyncService.processAndUploadProducts(bodyRequest.get("dataAreaId"));
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | JsonProcessingException e){
            System.err.println("An error occurred while synchronizing the products.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/synchronize-images")
    public String synchronizeImages(@RequestBody HashMap<String, String> request) {
        try {
            String itemsPerCall = MWUtils.bodyValidation(request.get("itemsPerCall")), authToken, merchantId;
            imageSyncService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            authToken = imageSyncService.getAccessToken();
            merchantId = imageSyncService.getMerchantId(authToken);
            return imageSyncService.processImagesSync(new AtomicReference<>(authToken), merchantId, Integer.parseInt(itemsPerCall));
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                 NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | JsonProcessingException e) {
            System.err.println("An error occurred while uploading the images.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/update-products-control-table")
    public String updateProductsControlTable(@RequestBody HashMap<String, String> interfaceDS){
        try{
            return ctrlTableService.processAndSaveInterfaceInfo(interfaceDS.get("interfaceId"), interfaceDS.get("dataAreaId"));
        } catch (RuntimeException e){
            System.err.println("An error occurred while updating product control table.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/update-interface-information")
    public String updateInterfaceInfo(@RequestBody HashMap<String, String> interfaceDS){
        try {
            return switch (interfaceDS.get("interfaceId")) {
                case "DYN" ->
                        interfaceInfoService.updateDYNInterfaceInfo(interfaceDS.get("dataAreaId"), interfaceDS.get("interfaceId"));
                case "UCA", "TVH" ->
                        interfaceInfoService.updateInterfaceInfo(interfaceDS.get("dataAreaId"), interfaceDS.get("interfaceId"));
                default -> throw new RuntimeException("The interfaceId sent doesn't exist.");
            };
        } catch (RuntimeException e){
            System.err.println("An error occurred while updating interface information.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/upload-item-inventory")
    public String uploadItemInventory(@RequestBody HashMap<String, String> requestBody){
        try {
            itemInventService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            itemInventService.uploadWarehouses(requestBody.get("dataAreaId"));
            return itemInventService.processItemInventoryUpdate(requestBody.get("dataAreaId"), Integer.parseInt(requestBody.get("itemsPerCall")));
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                 NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | JsonProcessingException e){
            System.err.println("An error occurred while uploading item inventory.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/upload-price-list")
    public String uploadPriceList(@RequestBody HashMap<String, String> requestBody) {
        try {
            SynchronizedPriceList priceList;
            String accessToken, merchantId, fullPriceListName, currencyId,
                    dataAreaId = MWUtils.bodyValidation(requestBody.get("dataAreaId")),
                    priceListName = MWUtils.bodyValidation(requestBody.get("priceListName")),
                    description = MWUtils.bodyValidation(requestBody.get("priceListDescription")),
                    marketplace = MWUtils.bodyValidation(requestBody.get("marketplace")),
                    currencyCode = MWUtils.bodyValidation(requestBody.get("currencyCode")),
                    itemsPerCall = MWUtils.bodyValidation(requestBody.get("itemsPerCall"));
            itemPriceSyncService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            accessToken = itemPriceSyncService.getAccessToken();
            merchantId = itemPriceSyncService.getMerchantId(accessToken);
            fullPriceListName = dataAreaId + "_" + priceListName + "_" + marketplace;
            currencyId = itemPriceSyncService.getCurrencyId(currencyCode, accessToken, merchantId);
            priceList = itemPriceSyncService.getSyncPriceList(fullPriceListName, currencyId, dataAreaId);
            if(priceList == null){
                JsonNode jsonNodePriceList =  itemPriceSyncService.createPriceList(fullPriceListName, description, currencyId, accessToken, merchantId);
                priceList = itemPriceSyncService.savePriceListInfo(jsonNodePriceList, dataAreaId);
            }
            return itemPriceSyncService.processPriceUpdate(priceList.getResponseId(), requestBody, Integer.parseInt(itemsPerCall), accessToken, dataAreaId);
        } catch(RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException | NoSuchAlgorithmException |
                BadPaddingException | InvalidKeyException | JsonProcessingException e){
            System.err.println("An error occurred while creating the price list.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/add-tvh-additional-info")
    public String uploadTVHAdditionalInfo() {
        try{
            itemSyncService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            return itemSyncService.processTVHAdditionalInfo();
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
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

}
