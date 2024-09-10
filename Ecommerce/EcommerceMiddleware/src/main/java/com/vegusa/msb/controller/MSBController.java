package com.vegusa.msb.controller;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.vegusa.middleware.entity.Company;
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
    public String uploadProducts(@RequestBody HashMap<String, String> request) {
        try {
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId"));
            Company company = itemSyncService.getCompany(dataAreaId);
            assert company != null : "The company provided doesn't exist.";
            itemSyncService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            return itemSyncService.processAndUploadProducts(dataAreaId);
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
    public String updateProductsControlTable(@RequestBody HashMap<String, String> request){
        try{
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId")),
                    interfaceId =  MWUtils.bodyValidation(request.get("interfaceId"));
            Company company = ctrlTableService.getCompany(dataAreaId);
            assert company != null : "The company provided doesn't exist.";
            return ctrlTableService.processAndSaveInterfaceInfo(interfaceId, dataAreaId);
        } catch (RuntimeException e){
            System.err.println("An error occurred while updating product control table.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/update-interface-information")
    public String updateInterfaceInfo(@RequestBody HashMap<String, String> request){
        try {
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId")),
                    interfaceId = MWUtils.bodyValidation(request.get("interfaceId"));
            Company company = interfaceInfoService.getCompany(dataAreaId);
            assert company != null : "The company provided doesn't exist.";
            return switch (interfaceId) {
                case "DYN" ->
                        interfaceInfoService.updateDYNInterfaceInfo(dataAreaId, interfaceId);
                case "UCA", "TVH" ->
                        interfaceInfoService.updateInterfaceInfo(dataAreaId, interfaceId);
                default -> throw new RuntimeException("The interfaceId sent doesn't exist.");
            };
        } catch (RuntimeException e){
            System.err.println("An error occurred while updating interface information.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/upload-item-inventory")
    public String uploadItemInventory(@RequestBody HashMap<String, String> request){
        try {
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId")),
                    itemsPerCall = MWUtils.bodyValidation(request.get("itemsPerCall"));
            Company company = itemInventService.getCompany(dataAreaId);
            assert company != null : "The company provided doesn't exist.";
            itemInventService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            itemInventService.uploadWarehouses(dataAreaId);
            return itemInventService.processItemInventoryUpdate(dataAreaId, Integer.parseInt(itemsPerCall));
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                 NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | JsonProcessingException e){
            System.err.println("An error occurred while uploading item inventory.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/synchronize-price-list")
    public String synchronizePriceList(@RequestBody HashMap<String, String> request) {
        try {
            SynchronizedPriceList priceList;
            String accessToken, merchantId, fullPriceListName, currencyId,
                    dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId")),
                    priceListName = MWUtils.bodyValidation(request.get("priceListName")),
                    description = MWUtils.bodyValidation(request.get("priceListDescription")),
                    channel = MWUtils.bodyValidation(request.get("channel")),
                    currencyCode = MWUtils.bodyValidation(request.get("currencyCode")),
                    itemsPerCall = MWUtils.bodyValidation(request.get("itemsPerCall"));
            Company company = itemPriceSyncService.getCompany(dataAreaId);
            assert company != null : "The company provided doesn't exist.";
            itemPriceSyncService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            accessToken = itemPriceSyncService.getAccessToken();
            merchantId = itemPriceSyncService.getMerchantId(accessToken);
            fullPriceListName = dataAreaId + "_" + priceListName + "_" + channel + "_" + currencyCode;
            currencyId = itemPriceSyncService.getCurrencyId(currencyCode, accessToken, merchantId);
            priceList = itemPriceSyncService.getSyncPriceList(fullPriceListName, currencyId, dataAreaId);
            if(priceList == null){
                String createdPriceList = itemPriceSyncService.createPriceList(fullPriceListName, description, currencyId, accessToken, merchantId);
                priceList = itemPriceSyncService.savePriceListInfo(createdPriceList, company);
            }
            return itemPriceSyncService.processPriceListSync(priceList, priceListName, channel, currencyCode, Integer.parseInt(itemsPerCall), accessToken);
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
