package com.vegusa.middleware.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.repository.*;
import com.vegusa.msb.entity.ItemInventLocation;
import com.vegusa.msb.repository.ItemInventLocationRepository;
import com.vegusa.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.middleware.utils.MWUtils;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.core.env.Environment;
import org.springframework.http.HttpHeaders;
import org.springframework.stereotype.Service;
import org.springframework.util.LinkedMultiValueMap;
import org.springframework.util.MultiValueMap;
import org.springframework.web.reactive.function.BodyInserters;
import org.springframework.web.reactive.function.client.WebClient;
import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;

@Service
public class ItemInventorySyncService {
    private final VegEcomvIntegrationEndptsRepository endpoints;
    private final ItemInventLocationRepository itemInventory;
    private final TokenInfoRepository tokenInfo;
    private final CompanyRepository companies;
    private final SyncProductsRepository syncProducts;
    private final SyncWarehouseRepository syncWarehouses;
    private final SyncItemInventoryRepository syncItemInventory;
    private final WebClient webClient;
    private final Environment env;
    private EncryptDecryptInterface encryptDecryptInterface;
    private String algorithm;

    @Autowired
    public ItemInventorySyncService(VegEcomvIntegrationEndptsRepository endpoints,
                                    ItemInventLocationRepository itemInventory,
                                    TokenInfoRepository tokenInfo,
                                    CompanyRepository companies,
                                    SyncProductsRepository syncProducts,
                                    SyncWarehouseRepository syncWarehouses,
                                    SyncItemInventoryRepository syncItemInventory,
                                    WebClient webClient,
                                    Environment env){
        this.endpoints = endpoints;
        this.itemInventory = itemInventory;
        this.tokenInfo = tokenInfo;
        this.companies = companies;
        this.syncProducts = syncProducts;
        this.syncWarehouses = syncWarehouses;
        this.syncItemInventory = syncItemInventory;
        this.webClient = webClient;
        this.env = env;
    }

    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface, String algorithm) {
        this.encryptDecryptInterface = encryptDecryptInterface;
        this.algorithm = algorithm;
    }

    public void uploadWarehouses(String dataAreaId) throws InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException,
            NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException {
        String accessToken = MWUtils.getDecryptedAccessToken(tokenInfo, encryptDecryptInterface, env, algorithm);
        JsonNode jsonNodeAppInfo = MWUtils.validateResponse("An error occurred while obtaining App Information: ",
                MWUtils.getAppInfo(webClient, endpoints.getIntegrationEndPoint("GET_APP_INFORMATION", "MULTIVENDE"), accessToken));
        String url = endpoints.getIntegrationEndPoint("CREATE_STORE_OR_WAREHOUSE", "MULTIVENDE")
                .replace("{{merchant_id}}", jsonNodeAppInfo.get("MerchantId").asText());
        Company company = companies.getCompany(dataAreaId);
        SynchronizedWarehouse syncWarehouse;
        JsonNode jsonNodeSyncWarehouses;
        String auxWarehouse = "";
        List<Object[]> warehouses = itemInventory.getWarehouses();
        for(Object[] warehouse : warehouses){
            try {
                auxWarehouse = warehouse[0] != null ? warehouse[0].toString() : "";
                syncWarehouse = syncWarehouses.getSyncWarehouse(auxWarehouse);
                if(syncWarehouse == null){
                    jsonNodeSyncWarehouses = MWUtils
                            .validateResponse("", syncWarehouses(accessToken, url, warehouse));
                    updateMiddlewareSyncWarehouses(jsonNodeSyncWarehouses, company);
                }
            } catch (RuntimeException | JsonProcessingException e){
                System.err.println("Error while uploading the warehouse " + auxWarehouse + " " + e.getMessage());
            }
        }
        System.out.println("The uploading of the warehouses is completed.");
    }

    private String syncWarehouses(String accessToken, String url, Object[] warehouse) throws RuntimeException {
        String name = warehouse[0] != null ? warehouse[0].toString() : "";
        String description = warehouse[1] != null ? warehouse[1].toString() : "";
        String address = warehouse[2] != null ? warehouse[2].toString() : "";
        HttpHeaders headers = new HttpHeaders();
        headers.add("Content-Type", "application/json");
        headers.add("Authorization", "Bearer " + accessToken);
        MultiValueMap<String, String> bodyValues = new LinkedMultiValueMap<>();
        bodyValues.add("name", name);
        bodyValues.add("type", "warehouse");
        bodyValues.add("description", description);
        bodyValues.add("address", address);
        return webClient.post()
                .uri(url)
                .headers(h -> h.addAll(headers))
                .body(BodyInserters.fromFormData(bodyValues))
                .retrieve()
                .bodyToMono(String.class)
                .block();
    }

    private void updateMiddlewareSyncWarehouses(JsonNode jsonNodeResp, Company company) throws RuntimeException, JsonProcessingException {
        ObjectMapper objMapSyncProducts = new ObjectMapper();
        SynchronizedWarehouse syncWarehouse = objMapSyncProducts.readValue(jsonNodeResp.toString(), SynchronizedWarehouse.class);
        syncWarehouse.setCompany(company);
        syncWarehouses.save(syncWarehouse);
    }

    public String processItemInventoryUpdate(String dataAreaId, int itemsPerCall) throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException {
        JSONObject response = new JSONObject();
        String accessToken = MWUtils.getDecryptedAccessToken(tokenInfo, encryptDecryptInterface, env, algorithm);
        Company company = companies.getCompany(dataAreaId);
        String url = endpoints.getIntegrationEndPoint("BULK_UPDATE_STOCK", "MULTIVENDE"), auxUrl = "";
        SynchronizedWarehouse[] syncWarehouses = this.syncWarehouses.getSyncWarehouses();
        ItemInventLocation[] auxItemInventory;
        List<JSONArray> auxBodyRequest;
        JsonNode auxSyncStock;
        for(SynchronizedWarehouse syncWarehouse : syncWarehouses){
            try{
                auxUrl = url.replace("{{warehouse_id}}", syncWarehouse.getIdEcom());
                auxItemInventory = itemInventory.getItemInventLocation(syncWarehouse.getName());
                auxBodyRequest = getBodyReqInventoryUpdate(auxItemInventory, syncWarehouse.getName(), itemsPerCall);
                for (JSONArray itemsStock : auxBodyRequest) {
                    try {
                        auxSyncStock = MWUtils.validateResponse("", synStock(accessToken, auxUrl, itemsStock));
                        updateSyncItemStockDB(auxSyncStock, company);
                        response.accumulate("UpdateAnswer", new JSONArray(auxSyncStock.toString()));
                        System.out.println("Stock successfully updated to " + syncWarehouse.getName());
                    } catch (RuntimeException | JsonProcessingException e){
                        response.accumulate("error", "Error while updating stock to " + syncWarehouse.getName() + " - " + e.getMessage());
                        System.err.println("Error while updating stock to " + syncWarehouse.getName() + " - " + e.getMessage());
                    }
                }
            }catch (RuntimeException e){
                System.err.println(e.getMessage());
            }
        }
        return response.toString();
    }

    private List<JSONArray> getBodyReqInventoryUpdate(ItemInventLocation[] itemInventLocation, String warehouse, int itemsPerCall) throws RuntimeException{
        List<JSONArray> response = new ArrayList<JSONArray>();
        JSONArray auxItemInventoryArray = new JSONArray();
        JSONObject auxItemInventoryObj;
        SynchronizedProducts auxSyncProduct;
        int auxCountItemArray = 0, auxItemInventory = 0;
        for (ItemInventLocation itemInventory : itemInventLocation) {
            try{
                auxItemInventory++;
                auxSyncProduct = syncProducts.getSynchronizedProductById(itemInventory.getId().getArticulo());
                if (auxSyncProduct != null) {
                    auxCountItemArray++;
                    auxItemInventoryObj = new JSONObject();
                    auxItemInventoryObj.put("code", auxSyncProduct.getDefaultVersionId());
                    auxItemInventoryObj.put("amount", itemInventory.getDisponible());
                    auxItemInventoryArray.put(auxItemInventoryObj);
                    if (auxCountItemArray % itemsPerCall == 0) {
                        response.add(auxItemInventoryArray);
                        auxItemInventoryArray = new JSONArray();
                        auxCountItemArray = 0;
                    }
                }
                if(auxItemInventory == itemInventLocation.length && !auxItemInventoryArray.isEmpty()){
                    response.add(auxItemInventoryArray);
                }
            }catch(RuntimeException e){
                System.err.println("An error occurred while obtaining body request value to " + itemInventory.getId().getArticulo() + " / " + warehouse);
            }
        }
        return response;
    }

    private String synStock(String accessToken, String url, JSONArray bodyValues) throws RuntimeException{
        HttpHeaders headers = new HttpHeaders();
        headers.add("Content-Type", "application/json");
        headers.add("Authorization", "Bearer " + accessToken);
        return webClient.post()
                .uri(url)
                .headers(h -> h.addAll(headers))
                .bodyValue(bodyValues.toString())
                .retrieve()
                .bodyToMono(String.class)
                .block();
    }

    private void updateSyncItemStockDB(JsonNode jsonNodeResp, Company company) throws RuntimeException, JsonProcessingException {
        JSONArray response = new JSONArray(jsonNodeResp.toString());
        SynchronizedItemInventory syncItemInventory, auxSyncItemInventory;
        SyncItemInventory readItemInventory = new SyncItemInventory();
        ObjectMapper objMapSyncStock = new ObjectMapper();
        JSONObject auxSyncStockObject;
        String auxProdRelId = "", auxProdRelAmount = "", auxProdRelType = "", auxProdRelCategoryId = "", auxAvailableProdStockId = "", auxAvailableProdStockAmount = "";
        for (int it = 0; it < response.length(); it++){
            try {
                auxSyncStockObject = response.getJSONObject(it);
                readItemInventory = objMapSyncStock.readValue(auxSyncStockObject.toString(), SyncItemInventory.class);
                if(!Objects.equals(readItemInventory.getProductRelocation().asText(), "null")){
                    auxProdRelId = readItemInventory.getProductRelocation().get("_id").asText();
                    auxProdRelAmount = readItemInventory.getProductRelocation().get("amount").asText();
                    auxProdRelType = readItemInventory.getProductRelocation().get("type").asText();
                    auxProdRelCategoryId = readItemInventory.getProductRelocation().get("ProductRelocationCategoryId").asText();
                }
                if(!Objects.equals(readItemInventory.getAvailableProductStock().asText(), "null")){
                    auxAvailableProdStockId = readItemInventory.getAvailableProductStock().get("_id").asText();
                    auxAvailableProdStockAmount = readItemInventory.getAvailableProductStock().get("amount").asText();
                }
                auxSyncItemInventory =  this.syncItemInventory.getSyncItemInventory(readItemInventory.getIdEcom());
                syncItemInventory = auxSyncItemInventory == null ? new SynchronizedItemInventory() : auxSyncItemInventory;
                syncItemInventory.setCode(readItemInventory.getCode());
                syncItemInventory.setSuccess(readItemInventory.getSuccess());
                if (Objects.equals(readItemInventory.getSuccess(), "true")) {
                    syncItemInventory.setIdEcom(readItemInventory.getIdEcom());
                    syncItemInventory.setWarehouseId(readItemInventory.getWarehouseId());
                    syncItemInventory.setItemRelocationId(auxProdRelId);
                    syncItemInventory.setItemRelocationAmount(auxProdRelAmount);
                    syncItemInventory.setItemRelocationType(auxProdRelType);
                    syncItemInventory.setItemRelocationCategoryId(auxProdRelCategoryId);
                    syncItemInventory.setAvailableProdStockId(auxAvailableProdStockId);
                    syncItemInventory.setAvailableProdStockAmount(auxAvailableProdStockAmount);
                } else {
                    syncItemInventory.setErrorItemAmount(readItemInventory.getAmount());
                    syncItemInventory.setErrorMessage(readItemInventory.getError());
                }
                syncItemInventory.setCompany(company);
                this.syncItemInventory.save(syncItemInventory);
            } catch (RuntimeException e){
                System.err.println("Error while saving synchronized item inventory info for " +  readItemInventory.getCode());
            }
        }
    }
}

