package com.vegusa.msb.controller;
//
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

@RestController
@RequestMapping("msb-ecommerce-integration")
public class MSBController {
    private final ItemSyncService msbProductSyncService;
    private final ItemControlTableService controlTable;
    private final InterfaceInfoService interfaceInfo;
    private final ItemInventorySyncService itemInventory;
    private final ItemPriceSyncService itemPrice;
    private final ImageSyncService msbImagesSyncService;
    private final WebScraperService webScraperService;

    @Autowired
    public MSBController(ItemSyncService msbProductSyncService,
                         ItemControlTableService controlTable,
                         InterfaceInfoService interfaceInfo,
                         ItemInventorySyncService itemInventory,
                         ItemPriceSyncService itemPrice,
                         ImageSyncService msbImagesSyncService,
                         WebScraperService webScraperService){
        this.msbProductSyncService = msbProductSyncService;
        this.controlTable = controlTable;
        this.interfaceInfo = interfaceInfo;
        this.itemInventory = itemInventory;
        this.itemPrice = itemPrice;
        this.msbImagesSyncService = msbImagesSyncService;
        this.webScraperService = webScraperService;
    }

    @PostMapping(value = "/synchronize-products")
    public String uploadProducts(@RequestBody HashMap<String, String> bodyRequest) {
        try {
            msbProductSyncService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            return msbProductSyncService.processAndUploadProducts(bodyRequest.get("dataAreaId"));
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
            msbImagesSyncService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
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
        try{
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

    @PostMapping(value="/upload-item-inventory")
    public String uploadItemInventory(@RequestBody HashMap<String, String> requestBody){
        try {
            itemInventory.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            itemInventory.uploadWarehouses(requestBody.get("dataAreaId"));
            return itemInventory.processItemInventoryUpdate(requestBody.get("dataAreaId"), Integer.parseInt(requestBody.get("itemsPerCall")));
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                 NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | JsonProcessingException e){
            System.err.println("An error occurred while uploading item inventory.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
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
            itemPrice.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            accessToken = itemPrice.getAccessToken();
            merchantId = itemPrice.getMerchantId(accessToken);
            fullPriceListName = dataAreaId + "_" + priceListName + "_" + marketplace;
            currencyId = itemPrice.getCurrencyId(currencyCode, accessToken, merchantId);
            priceList = itemPrice.getSyncPriceList(fullPriceListName, currencyId, dataAreaId);
            if(priceList == null){
                JsonNode jsonNodePriceList =  itemPrice.createPriceList(fullPriceListName, description, currencyId, accessToken, merchantId);
                priceList = itemPrice.savePriceListInfo(jsonNodePriceList, dataAreaId);
            }
            return itemPrice.processPriceUpdate(priceList.getResponseId(), requestBody, Integer.parseInt(itemsPerCall), accessToken, dataAreaId);
        } catch(RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException | NoSuchAlgorithmException |
                BadPaddingException | InvalidKeyException | JsonProcessingException e){
            System.err.println("An error occurred while creating the price list.");
            System.err.println("StackTrace: ");
            e.printStackTrace();
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/add-tvh-additional-info")
    public String uploadTVHAdditionalInfo() {
        try{
            msbProductSyncService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
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
