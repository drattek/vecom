package com.vegusa.msb.controller;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.vegusa.middleware.entity.Company;
import com.vegusa.middleware.entity.SyncPriceList;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerCategoryDto;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerProductDto;
import com.vegusa.middleware.integrations.jumpseller.service.JumpsellerCategoryService;
import com.vegusa.middleware.integrations.jumpseller.service.JumpsellerProductService;
import com.vegusa.middleware.service.*;
import com.vegusa.middleware.utils.MWUtils;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;
import reactor.core.publisher.Mono;

import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.util.HashMap;

@RestController
@RequestMapping("msb-ecommerce-middleware")
public class MSBController {
    private final ItemSyncService itemSyncService;
    private final ItemControlTableService ctrlTableService;
    private final InterfaceInfoService interfaceInfoService;
    private final ItemInventorySyncService itemInventService;
    private final ItemPriceSyncService itemPriceSyncService;
    private final ImageSyncService imageSyncService;
    private final JumpsellerProductService jumpsellerService;
    private final JumpsellerCategoryService jumpsellerCategoryService;

    private final WebScraperService webScraperService;

    @Autowired
    public MSBController(ItemSyncService itemSyncService,
                         ItemControlTableService ctrlTableService,
                         InterfaceInfoService interfaceInfoService,
                         ItemInventorySyncService itemInventService,
                         ItemPriceSyncService itemPriceSyncService,
                         ImageSyncService imageSyncService,
                         JumpsellerProductService jumpsellerService,
                         JumpsellerCategoryService jumpsellerCategoryService,
                         WebScraperService webScraperService){
        this.itemSyncService = itemSyncService;
        this.ctrlTableService = ctrlTableService;
        this.interfaceInfoService = interfaceInfoService;
        this.itemInventService = itemInventService;
        this.itemPriceSyncService = itemPriceSyncService;
        this.imageSyncService = imageSyncService;
        this.webScraperService = webScraperService;
        this.jumpsellerService = jumpsellerService;
        this.jumpsellerCategoryService = jumpsellerCategoryService;
    }

    /*
    @GetMapping(value = "/get-products")
    public Mono<List<JumpsellerProductDto>> getAllProducts(@RequestParam(defaultValue = "1") int page) {
        try {
            return jumpsellerService.getAllProducts();
        } catch (RuntimeException e) {
            System.err.println(e.getMessage());
        }
        return null;
    }
    */

    @PostMapping(value = "/update-prices-jumpseller")
    public Mono<ResponseEntity<String>> updatePricesJumpseller (@RequestBody HashMap<String, String> request){
        try {
            System.out.println("Start updating prices");
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId")),
                    products = MWUtils.bodyValidation(request.get("productList")); // Read the array of products uploaded
            JSONArray productsList = new JSONObject(products).getJSONArray("content"); // Transform the productlist to JsonArray
            jumpsellerService.updatePrices(productsList, dataAreaId).subscribe();

            return Mono.just(ResponseEntity.accepted().body("Update started"));
        } catch (RuntimeException e) {
            System.err.println(e.getMessage());
        }
        return null;
    }

    @PostMapping(value = "/update-products")
    public String updateProducts(@RequestBody HashMap<String, String> request) {
        try {
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId")), authToken, merchantId;
            Company company = itemSyncService.getCompany(dataAreaId);
            if(company == null){throw new RuntimeException("The company provided doesn't exist."); }
            itemSyncService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            authToken = itemSyncService.getAccessToken();
            merchantId = imageSyncService.getMerchantId(authToken);
            return itemSyncService.processProductsUpdate(authToken, merchantId, dataAreaId, company);
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | JsonProcessingException e){
            System.err.println("An error occurred while synchronizing the products.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/update-control-table-products-information")
    public String updateControlTableInfo(@RequestBody HashMap<String, String> request){
        try{
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId")),
                    interfaceId =  MWUtils.bodyValidation(request.get("interfaceId"));
            Company company = ctrlTableService.getCompany(dataAreaId);
            if(company == null){throw new RuntimeException("The company provided doesn't exist."); }
            return ctrlTableService.updateControlTableInfo(interfaceId, dataAreaId);
        } catch (RuntimeException e){
            System.err.println("An error occurred while updating product control table.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/update-interface-products-information")
    public String updateInterfaceInfo(@RequestBody HashMap<String, String> request){
        try {
            String dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId")),
                    interfaceId = MWUtils.bodyValidation(request.get("interfaceId"));
            Company company = interfaceInfoService.getCompany(dataAreaId);
            if(company == null){throw new RuntimeException("The company provided doesn't exist."); }
            return switch (interfaceId) {
                case "DYN" ->
                        interfaceInfoService.updateDYNInterfaceInfo(interfaceId, dataAreaId, company);
                case "UCA", "TVH", "BOB" ->
                        interfaceInfoService.updateInterfaceInfo(interfaceId, dataAreaId, company);
                default -> throw new RuntimeException("The interfaceId provided doesn't exist.");
            };
        } catch (RuntimeException e){
            System.err.println("An error occurred while updating interface information.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    //@Scheduled(fixedRateString = "${fixedRateRefreshItemInventory.in.milliseconds}", initialDelayString = "${fixedDelayRefreshItemInventory.in.milliseconds}")
    public String updateItemInventory(){
        try {
            System.out.println("Inventory Update Started.");
            String authToken, merchantId, dataAreaId = "MSB";//MWUtils.bodyValidation(request.get("dataAreaId")),
            int itemsPerCall = itemInventService.getProductsPerCall("UPDATE_STOCK_PRODUCTS_PER_CALL");//MWUtils.bodyValidation(request.get("itemsPerCall"));
            Company company = itemInventService.getCompany(dataAreaId);
            if(company == null){throw new RuntimeException("The company provided doesn't exist."); }
            itemInventService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            authToken = itemInventService.getAccessToken();
            merchantId = itemInventService.getMerchantId(authToken);
            itemInventService.uploadWarehouses(authToken, merchantId, company);
            return itemInventService.processItemInventoryUpdate(itemsPerCall, authToken, dataAreaId, company);
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                 NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | JsonProcessingException e){
            System.err.println("An error occurred while uploading item inventory.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    //@Scheduled(fixedRateString = "${fixedRateRefreshPriceLists.in.milliseconds}", initialDelayString = "${fixedDelayRefreshPriceLists.in.milliseconds}")
    public String updatePriceList() {
        try {
            System.out.println("Price Lists Update Started.");
            SyncPriceList priceList;
            String authToken, merchantId, fullPriceListName, currencyId,
                    dataAreaId = "MSB",//MWUtils.bodyValidation(request.get("dataAreaId")),
                    priceListName = "NORMAL",//MWUtils.bodyValidation(request.get("priceListName")),
                    description = "Lista de precios Normal Mercado Libre.", //MWUtils.bodyValidation(request.get("priceListDescription")),
                    channel = "MERCADO_LIBRE",//MWUtils.bodyValidation(request.get("channel")),
                    currencyCode = "MXN"; //MWUtils.bodyValidation(request.get("currencyCode"));
            int itemsPerCall = itemInventService.getProductsPerCall("UPDATE_PRICE_PRODUCTS_PER_CALL");//MWUtils.bodyValidation(request.get("itemsPerCall"));
            Company company = itemPriceSyncService.getCompany(dataAreaId);
            if(company == null){throw new RuntimeException("The company provided doesn't exist."); }
            itemPriceSyncService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            itemPriceSyncService.setAddFixedCostParameters("ADDITIONAL_FIXED_COST_LL", "ADDITIONAL_FIXED_COST_CL", "ADDITIONAL_FIXED_COST_UL", dataAreaId);
            authToken = itemPriceSyncService.getAccessToken();
            merchantId = itemPriceSyncService.getMerchantId(authToken);
            fullPriceListName = dataAreaId + "_" + priceListName + "_" + channel + "_" + currencyCode;
            currencyId = itemPriceSyncService.getCurrencyId(currencyCode, authToken, merchantId);
            priceList = itemPriceSyncService.getSyncPriceList(fullPriceListName, currencyId, dataAreaId);
            if (priceList == null) {
                String createdPriceList = itemPriceSyncService.createPriceList(fullPriceListName, description, currencyId, authToken, merchantId);
                priceList = itemPriceSyncService.savePriceListInfo(createdPriceList, company);
            }
            return itemPriceSyncService.processPriceListUpdate(priceList, priceListName, channel, currencyCode, itemsPerCall, authToken);
        } catch(RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException | NoSuchAlgorithmException |
                BadPaddingException | InvalidKeyException | JsonProcessingException e){
            System.err.println("An error occurred while creating the price list.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    @PostMapping(value="/upload-images")
    public String uploadImages(@RequestBody HashMap<String, String> request) {
        try {
            String itemsPerCall = MWUtils.bodyValidation(request.get("itemsPerCall")),
                    dataAreaId = MWUtils.bodyValidation(request.get("dataAreaId")),
                    albumId = request.get("albumId") == null ? "default" : request.get("albumId"),
                    authToken, merchantId;
            Company company = imageSyncService.getCompany(dataAreaId);
            if(company == null){throw new RuntimeException("The company provided doesn't exist."); }
            imageSyncService.setEncryptDecryptInterface(MWUtils.getEncryptDecryptInterface(), "AES/CBC/PKCS5Padding");
            authToken = imageSyncService.getAccessToken();
            merchantId = imageSyncService.getMerchantId(authToken);
            return imageSyncService.processImagesUpload(Integer.parseInt(itemsPerCall), authToken, merchantId, dataAreaId, company, albumId);
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                 NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | JsonProcessingException e) {
            System.err.println("An error occurred while uploading the images.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    /*
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
  */
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

    /*
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
    */


}
