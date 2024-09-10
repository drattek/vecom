package com.vegusa.middleware.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.repository.*;
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
import java.util.*;

@Service
public class ItemPriceSyncService {
    private final SyncPriceListRepository priceListRepo;
    private final SyncItemPriceRepository itemPriceRepo;
    private final CategoryRepository profitMargin;
    private final ItemInventLocationRepository itemInventoryRepo;
    private final SyncProductsRepository syncItemRepo;
    private final CompanyRepository companyRepo;
    private final EndpointRepository endpointRepo;
    private final AuthTokenRepository authTokenRepo;
    private final WebClient webClient;
    private final Environment env;
    private EncryptDecryptInterface encryptDecryptInterface;
    private String algorithm;

    @Autowired
    private ItemPriceSyncService(SyncPriceListRepository priceListRepo,
                                 SyncItemPriceRepository itemPriceRepo,
                                 CategoryRepository profitMargin,
                                 ItemInventLocationRepository itemInventoryRepo,
                                 SyncProductsRepository syncItemRepo,
                                 CompanyRepository companyRepo,
                                 EndpointRepository endpointRepo,
                                 AuthTokenRepository authTokenRepo,
                                 WebClient webClient,
                                 Environment env){
        this.priceListRepo = priceListRepo;
        this.itemPriceRepo = itemPriceRepo;
        this.profitMargin = profitMargin;
        this.itemInventoryRepo = itemInventoryRepo;
        this.syncItemRepo = syncItemRepo;
        this.companyRepo = companyRepo;
        this.endpointRepo = endpointRepo;
        this.authTokenRepo = authTokenRepo;
        this.webClient = webClient;
        this.env = env;
    }

    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface, String algorithm) {
        this.encryptDecryptInterface = encryptDecryptInterface;
        this.algorithm = algorithm;
    }

    public String getAccessToken() throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException {
        return MWUtils.getDecryptedAccessToken(authTokenRepo, encryptDecryptInterface, env, algorithm);
    }

    public String getMerchantId(String accessToken) throws RuntimeException, JsonProcessingException {
        String url = endpointRepo.getEndpointUrl("GET_APP_INFORMATION", env.getProperty("integration.company.name"));
        String appInfo = MWUtils.getAppInfo(webClient, url, accessToken);
        return MWUtils.getJsonNodeResponse(appInfo, "MerchantId");
    }

    public SynchronizedPriceList getSyncPriceList(String name, String currencyId, String dataAreaId) throws RuntimeException {
        return priceListRepo.getSyncPriceList(name, currencyId, dataAreaId);
    }

    public String createPriceList(String name, String description, String currencyId, String accessToken, String merchantId) {
        try {
            String url = endpointRepo.getEndpointUrl("CREATE_PRICE_LIST", env.getProperty("integration.company.name")).replace("{{merchant_id}}", merchantId);
            HttpHeaders headers = MWUtils.getHeaders(accessToken);
            MultiValueMap<String, String> bodyValues = new LinkedMultiValueMap<>();
            bodyValues.add("name", name);
            bodyValues.add("description", description);
            bodyValues.add("CurrencyId", currencyId);
            return webClient.post()
                            .uri(url)
                            .headers(h -> h.addAll(headers))
                            .body(BodyInserters.fromFormData(bodyValues))
                            .retrieve()
                            .bodyToMono(String.class)
                            .block();
        }catch (RuntimeException e){
            throw new RuntimeException("An error occurred while creating the price list: " + e.getMessage());
        }
    }

    public SynchronizedPriceList savePriceListInfo(String response, String dataAreaId) throws RuntimeException {
        try {
            Company company = companyRepo.getCompany(dataAreaId);
            ObjectMapper objMapPriceList = new ObjectMapper();
            SynchronizedPriceList priceList = objMapPriceList.readValue(response, SynchronizedPriceList.class);
            priceList.setCompany(company);
            priceListRepo.save(priceList);
            return priceList;
        } catch (RuntimeException | JsonProcessingException e){
            throw new RuntimeException("An error occurred while saving information of price list created.");
        }
    }

    public String getCurrencyId(String currencyCode, String accessToken, String merchantId) {
        try {
            String url, allCurrencies;
            HttpHeaders headers = MWUtils.getHeaders(accessToken);
            url = endpointRepo.getEndpointUrl("GET_CURRENCIES", env.getProperty("integration.company.name")).replace("{{merchant_id}}", merchantId);
            allCurrencies = webClient.get()
                    .uri(url)
                    .headers(h -> h.addAll(headers))
                    .retrieve()
                    .bodyToMono(String.class)
                    .block();
            return selectCurrencyId(currencyCode, allCurrencies);
        } catch (RuntimeException e){
            throw new RuntimeException("An error occurred while obtaining the Currency Id for the given code: " + e.getMessage());
        }
    }

    private String selectCurrencyId(String currencyCode, String allCurrencies) throws RuntimeException {
        String response = "";
        JSONArray responseArray, auxCurrencies;
        JSONObject auxJsonObj;
        auxJsonObj = new JSONObject(allCurrencies);
        responseArray = new JSONArray(auxJsonObj.get("entries").toString());
        for (int it = 0; it < responseArray.length(); it++){
            auxJsonObj = responseArray.getJSONObject(it);
            if(Objects.equals(auxJsonObj.get("code").toString(), currencyCode)){
                auxCurrencies = new JSONArray(auxJsonObj.get("Currencies").toString());
                auxJsonObj = auxCurrencies.getJSONObject(0);
                response = auxJsonObj.get("_id").toString();
            }
        }
        if(Objects.equals(response, "")){
            throw new RuntimeException("No Currency Id found for the currency code provided.");
        }
        return response;
    }

    public String processPriceListSync(String priceListId, HashMap<String, String> requestBody, int itemsPerCall, String accessToken, String dataAreaId)
            throws RuntimeException, JsonProcessingException {
        JSONObject response = new JSONObject();
        Company company = companyRepo.getCompany(dataAreaId);
        String url = endpointRepo.getEndpointUrl("UPDATE_PRICE_BULK_SET", env.getProperty("integration.company.name"))
                .replace("{{product_price_list_id}}", priceListId);
        List<JSONArray> bodyValues = getItemPrices(requestBody, itemsPerCall, dataAreaId);
        for(JSONArray bodyValue: bodyValues){
            try {
                String updatedPrices = updatePrices(accessToken, url, bodyValue);
                saveSyncPriceListInfo(updatedPrices, priceListId, company);
                response.accumulate("UpdatePrices", new JSONArray(updatedPrices));
                System.out.println("A part of the price list was successfully updated.");
            }catch (RuntimeException e){
                response.accumulate("error", e.getMessage());
                System.err.println("An error occurred while updating a part of price list.");
            }
        }
        return response.toString();
    }

    private List<JSONArray> getItemPrices(HashMap<String, String> requestBody, int itemsPerCall, String dataAreaId) throws RuntimeException {
        List<JSONArray> response = new ArrayList<>();
        JSONArray auxItemPrices = new JSONArray();
        float basePercentage = getBasePercentage(requestBody, dataAreaId), itemPercentage = 0, auxCost = 0;
        HashMap<String, String> itemCost = getItemMap(itemInventoryRepo.getItemCost());
        HashMap<String, String> itemCategory = getItemMap(itemInventoryRepo.getItemCategory());
        SynchronizedProducts[] syncProducts = this.syncItemRepo.getSynchronizedProducts();
        int countItemSyncProducts = 0, countItemArray = 0;
        for(SynchronizedProducts product : syncProducts){
            countItemSyncProducts++;
            try {
               if(itemCost.get(product.getInternalCode()) != null && itemCategory.get(product.getInternalCode()) != null ) {
                   countItemArray++;
                   itemPercentage = profitMargin.getCategoryPercentage(itemCategory.get(product.getInternalCode()), requestBody.get("currencyCode"), dataAreaId).getPercentage().floatValue();
                    auxCost = (((basePercentage + itemPercentage) / 100) + 1) * Float.parseFloat(itemCost.get(product.getInternalCode()));
                    JSONObject itemPrice = new JSONObject();
                    itemPrice.put("ProductVersionId", product.getDefaultVersionId());
                    itemPrice.put("gross", "");
                    itemPrice.put("priceWithDiscount", "");
                    itemPrice.put("tax", "");
                    itemPrice.put("net", auxCost);
                    auxItemPrices.put(itemPrice);
                   if (countItemArray % itemsPerCall == 0) {
                       response.add(auxItemPrices);
                       auxItemPrices = new JSONArray();
                       countItemArray = 0;
                   }
                    System.out.println("GENERO ALGO ");
                }
                if(countItemSyncProducts == syncProducts.length && !auxItemPrices.isEmpty()){
                    response.add(auxItemPrices);
                }
                System.out.println("TERMINO BIEN " + countItemSyncProducts + "/" + countItemArray);
            } catch (RuntimeException e){
                System.err.println("OCURRIO UN ERROR" + e.getMessage());
            }
        }
        return response;
    }

    private HashMap<String, String> getItemMap(List<Object[]> itemCosts){
        HashMap<String, String> response = new HashMap<>();
        for(Object[] itemCost: itemCosts){
            try {
                response.put(itemCost[0].toString(), itemCost[1].toString());
            }catch (RuntimeException e){
                System.err.println("An error occurred while saving Item Cost Map.");
            }
        }
        return  response;
    }

    private float getBasePercentage(HashMap<String, String> requestBody, String dataAreaId) throws RuntimeException{
     //   ProfitMarginCategory[] marginCategories = this.marginCategories.getProfitMarginCategory(false, true);
        HashMap<String, String> bodyReqRelation = MWUtils.getMarginCategoriesBodyRelation();
        String auxCategoryName = "";
        float response = 0;
    /*    for(ProfitMarginCategory category : marginCategories){
            auxCategoryName = category.getId().getName();
            response += profitMargin.getProfitMargin(auxCategoryName, requestBody.get(bodyReqRelation.get(auxCategoryName)),
                    requestBody.get("currencyCode"), dataAreaId).getPercentage().floatValue();
        } */
        return response;
    }

    private String updatePrices(String accessToken, String url, JSONArray bodyValues) {
        try {
            HttpHeaders headers = MWUtils.getHeaders(accessToken);
            return webClient.post()
                    .uri(url)
                    .headers(h -> h.addAll(headers))
                    .bodyValue(bodyValues.toString())
                    .retrieve()
                    .bodyToMono(String.class)
                    .block();
        } catch (RuntimeException e) {
            throw new RuntimeException("An error occurred while updating Price List: " + e.getMessage());
        }
    }

    private void saveSyncPriceListInfo(String updatedPrices, String priceListId, Company company) throws RuntimeException {
        JSONArray response = new JSONArray(updatedPrices);
        ObjectMapper objMapSyncItemPrice = new ObjectMapper();
        JSONObject auxSyncItemPriceObj;
        SyncItemPrice auxSyncItemPrice;
        for(int it = 0; it < response.length(); it++){
            try {
                auxSyncItemPriceObj = response.getJSONObject(it);
                SyncItemPrice syncItemPrice = objMapSyncItemPrice.readValue(auxSyncItemPriceObj.toString(), SyncItemPrice.class);
                auxSyncItemPrice = itemPriceRepo.getSyncItemPrice(priceListId, syncItemPrice.getProductVersionId(), company.getId().getDataAreaId());
                syncItemPrice.setPriceListId(priceListId);
                syncItemPrice.setCompany(company);
                if(auxSyncItemPrice != null){
                    syncItemPrice.setId(auxSyncItemPrice.getId());
                }
                itemPriceRepo.save(syncItemPrice);
            } catch (RuntimeException | JsonProcessingException e){
                System.err.println("Error while saving synchronized item price info.");
            }
        }
    }

}
