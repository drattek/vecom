package com.vegusa.middleware.service;

import com.fasterxml.jackson.core.JsonProcessingException;
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
    private final ItemInventLocationRepository itemInventLocRepo;
    private final SyncItemInventoryRepository syncItemInventRepo;
    private final SyncItemRepository syncItemRepo;
    private final SyncWarehouseRepository syncWarehouseRepo;
    private final CompanyRepository companyRepo;
    private final EndpointRepository endpointRepo;
    private final AuthTokenRepository authTokenRepo;
    private final SystemParameterRepository systemParameterRepo;
    private final WebClient webClient;
    private final Environment env;
    private EncryptDecryptInterface encryptDecryptInterface;
    private String algorithm;

    @Autowired
    public ItemInventorySyncService(ItemInventLocationRepository itemInventLocRepo,
                                    SyncItemInventoryRepository syncItemInventRepo,
                                    SyncItemRepository syncItemRepo,
                                    SyncWarehouseRepository syncWarehouseRepo,
                                    CompanyRepository companyRepo,
                                    EndpointRepository endpointRepo,
                                    AuthTokenRepository authTokenRepo,
                                    SystemParameterRepository systemParameterRepo,
                                    WebClient webClient,
                                    Environment env){
        this.itemInventLocRepo = itemInventLocRepo;
        this.syncItemInventRepo = syncItemInventRepo;
        this.syncItemRepo = syncItemRepo;
        this.syncWarehouseRepo = syncWarehouseRepo;
        this.companyRepo = companyRepo;
        this.endpointRepo = endpointRepo;
        this.authTokenRepo = authTokenRepo;
        this.systemParameterRepo = systemParameterRepo;
        this.webClient = webClient;
        this.env = env;
    }

    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface, String algorithm) {
        this.encryptDecryptInterface = encryptDecryptInterface;
        this.algorithm = algorithm;
    }

    public String getAccessToken() throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException {
        AuthToken tokenInfo =  authTokenRepo.getAuthToken(env.getProperty("integration.company.name"));
        return MWUtils.getDecryptedAccessToken(tokenInfo, encryptDecryptInterface, algorithm);
    }

    public String getMerchantId(String accessToken) throws RuntimeException, JsonProcessingException {
        String url = endpointRepo.getEndpointUrl("GET_APP_INFORMATION", env.getProperty("integration.company.name"));
        String appInfo = MWUtils.getAppInfo(webClient, url, accessToken);
        return MWUtils.getJsonNodeResponse(appInfo, "MerchantId");
    }

    public Company getCompany(String dataAreaId) throws RuntimeException {
        return companyRepo.getCompany(dataAreaId);
    }

    public int getProductsPerCall(String sysParameterName) throws RuntimeException {
        return systemParameterRepo.getSystemParameter(sysParameterName).getIntValue();
    }

    public void uploadWarehouses(String authToken, String merchantId, Company company) throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException {
        String url = endpointRepo.getEndpointUrl("CREATE_STORE_OR_WAREHOUSE", env.getProperty("integration.company.name"))
                .replace("{{merchant_id}}", merchantId);
        SyncWarehouse syncWarehouse;
        String auxWarehouse = "";
        List<Object[]> warehouses = itemInventLocRepo.getWarehouse();
        for(Object[] warehouse : warehouses){
            try {
                auxWarehouse = warehouse[0] != null ? warehouse[0].toString() : "";
                syncWarehouse = syncWarehouseRepo.getSyncWarehouse(auxWarehouse);
                if(syncWarehouse == null){
                    String createWarehouseResp = createWarehouse(authToken, url, warehouse);
                    saveWarehouseInfo(createWarehouseResp, company);
                }
            } catch (RuntimeException | JsonProcessingException e){
                System.err.println("Error while uploading the warehouse " + auxWarehouse + " " + e.getMessage());
            }
        }
        System.out.println("The uploading of the warehouses is completed.");
    }

    private String createWarehouse(String accessToken, String url, Object[] warehouse) {
        try {
            String name = warehouse[0] != null ? warehouse[0].toString() : "";
            String description = warehouse[1] != null ? warehouse[1].toString() : "";
            String address = warehouse[2] != null ? warehouse[2].toString() : "";
            HttpHeaders headers = MWUtils.getHeaders(accessToken);
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
        } catch (RuntimeException e) {
            throw new RuntimeException("An error occurred while creating warehouse: " + e.getMessage());
        }
    }

    private void saveWarehouseInfo(String createWarehouseResp, Company company) throws RuntimeException, JsonProcessingException {
        ObjectMapper objMapSyncProducts = new ObjectMapper();
        SyncWarehouse syncWarehouse = objMapSyncProducts.readValue(createWarehouseResp, SyncWarehouse.class);
        syncWarehouse.setCompany(company);
        syncWarehouseRepo.save(syncWarehouse);
    }

    public String processItemInventoryUpdate(int itemsPerCall, String authToken, String dataAreaId, Company company) throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException {
        JSONObject response = new JSONObject();
        String url = endpointRepo.getEndpointUrl("BULK_UPDATE_STOCK", env.getProperty("integration.company.name")), auxUrl = "";
        SyncWarehouse[] syncWarehouses = this.syncWarehouseRepo.getSyncWarehouse();
        ItemInventLocation[] auxItemInventory;
        List<JSONArray> auxRequest;
        String updateStockResp;
        for(SyncWarehouse syncWarehouse : syncWarehouses){
            try{
                auxUrl = url.replace("{{warehouse_id}}", syncWarehouse.getIdEcom());
                auxItemInventory = itemInventLocRepo.getItemInventLocation(syncWarehouse.getName());
                auxRequest = getRequestItemInventory(auxItemInventory, syncWarehouse.getName(), itemsPerCall, dataAreaId);
                for (JSONArray itemsStock : auxRequest) {
                    try {
                        updateStockResp = updateStock(authToken, auxUrl, itemsStock);
                        saveStockInfo(updateStockResp, company);
                        response.accumulate("update", new JSONArray(updateStockResp));
                        System.out.println("Stock successfully updated to " + syncWarehouse.getName());
                    } catch (RuntimeException | JsonProcessingException e){
                        response.accumulate("error", "Error while updating stock to " + syncWarehouse.getName() + " " + e.getMessage());
                        System.err.println("Error while updating stock to " + syncWarehouse.getName() + " - " + e.getMessage());
                    }
                }
            }catch (RuntimeException e){
                System.err.println(e.getMessage());
            }
        }
        System.out.println("Inventory Update Ends.");
        return response.toString();
    }

    private List<JSONArray> getRequestItemInventory(ItemInventLocation[] itemInventLocation, String warehouse, int itemsPerCall, String dataAreaId) throws RuntimeException {
        List<JSONArray> response = new ArrayList<JSONArray>();
        JSONArray auxItemInventoryArray = new JSONArray();
        JSONObject auxItemInventoryObj;
        SyncItem auxSyncProduct;
        int countItemArray = 0, countItemInventory = 0;
        for (ItemInventLocation itemInventory : itemInventLocation) {
            try{
                countItemInventory++;
                auxSyncProduct = syncItemRepo.getSyncItem(itemInventory.getId().getArticulo(), dataAreaId);
                if (auxSyncProduct != null) {
                    countItemArray++;
                    auxItemInventoryObj = new JSONObject();
                    auxItemInventoryObj.put("code", auxSyncProduct.getDefaultVersionId());
                    auxItemInventoryObj.put("amount", itemInventory.getDisponible());
                    auxItemInventoryArray.put(auxItemInventoryObj);
                    if (countItemArray % itemsPerCall == 0) {
                        response.add(auxItemInventoryArray);
                        auxItemInventoryArray = new JSONArray();
                        countItemArray = 0;
                    }
                }
                if(countItemInventory == itemInventLocation.length && !auxItemInventoryArray.isEmpty()){
                    response.add(auxItemInventoryArray);
                }
            }catch(RuntimeException e){
                System.err.println("An error occurred while obtaining request value to " + itemInventory.getId().getArticulo() + " / " + warehouse);
            }
        }
        return response;
    }

    private String updateStock(String accessToken, String url, JSONArray itemsStock) {
        try {
            HttpHeaders headers = new HttpHeaders();
            headers.add("Content-Type", "application/json");
            headers.add("Authorization", "Bearer " + accessToken);
            return webClient.post()
                    .uri(url)
                    .headers(h -> h.addAll(headers))
                    .bodyValue(itemsStock.toString())
                    .retrieve()
                    .bodyToMono(String.class)
                    .block();
        } catch (RuntimeException e) {
            throw new RuntimeException("An error occurred while updating stock: " + e.getMessage());
        }
    }

    private void saveStockInfo(String updateStockInfo, Company company) throws RuntimeException, JsonProcessingException {
        JSONArray response = new JSONArray(updateStockInfo);
        SyncItemInventory syncItemInventory, auxSyncItemInventory;
        SyncItemInventoryReader readItemInventory = new SyncItemInventoryReader();
        ObjectMapper objMapSyncStock = new ObjectMapper();
        JSONObject syncStockObject;
        String auxProdRelId = "", auxProdRelAmount = "", auxProdRelType = "", auxProdRelCategoryId = "", auxAvailableProdStockId = "", auxAvailableProdStockAmount = "";
        for (int it = 0; it < response.length(); it++){
            try {
                syncStockObject = response.getJSONObject(it);
                readItemInventory = objMapSyncStock.readValue(syncStockObject.toString(), SyncItemInventoryReader.class);
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
                auxSyncItemInventory =  syncItemInventRepo.getSyncItemInventory(readItemInventory.getIdEcom());
                syncItemInventory = auxSyncItemInventory == null ? new SyncItemInventory() : auxSyncItemInventory;
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
                this.syncItemInventRepo.save(syncItemInventory);
            } catch (RuntimeException e){
                System.err.println("Error while saving synchronized item inventory info for " +  readItemInventory.getCode());
            }
        }
    }
}

