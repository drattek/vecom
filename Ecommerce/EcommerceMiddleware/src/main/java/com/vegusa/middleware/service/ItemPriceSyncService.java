package com.vegusa.middleware.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
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
    private final ItemInventLocationRepository itemInventory;
    private final EndpointRepository endpoints;
    private final AuthTokenRepository tokenInfo;
    private final SyncPriceListRepository priceLists;
    private final SyncItemPriceRepository itemPrices;
    private final ProfitMarginCategoryRepository marginCategories;
    private final ProfitMarginRepository profitMargin;
    private final CompanyRepository companies;
    private final SyncProductsRepository syncProducts;
    private final WebClient webClient;
    private final Environment env;
    private EncryptDecryptInterface encryptDecryptInterface;
    private String algorithm;

    @Autowired
    private ItemPriceSyncService(ItemInventLocationRepository itemInventory,
                                 EndpointRepository endpoints,
                                 AuthTokenRepository tokenInfo,
                                 SyncPriceListRepository priceLists,
                                 SyncItemPriceRepository itemPrices,
                                 ProfitMarginCategoryRepository marginCategories,
                                 ProfitMarginRepository profitMargin,
                                 CompanyRepository companies,
                                 SyncProductsRepository syncProducts,
                                 WebClient webClient,
                                 Environment env){
        this.itemInventory = itemInventory;
        this.endpoints = endpoints;
        this.tokenInfo = tokenInfo;
        this.priceLists = priceLists;
        this.itemPrices = itemPrices;
        this.marginCategories = marginCategories;
        this.profitMargin = profitMargin;
        this.companies = companies;
        this.syncProducts = syncProducts;
        this.webClient = webClient;
        this.env = env;
    }

    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface, String algorithm) {
        this.encryptDecryptInterface = encryptDecryptInterface;
        this.algorithm = algorithm;
    }

    public String getAccessToken() throws InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException,
            NoSuchAlgorithmException, BadPaddingException, InvalidKeyException {
        return MWUtils.getDecryptedAccessToken(tokenInfo, encryptDecryptInterface, env, algorithm);
    }

    public String getMerchantId(String accessToken) throws RuntimeException, JsonProcessingException {
        String url = endpoints.getEndpointUrl("GET_APP_INFORMATION", env.getProperty("integration.company.name"));
        String appInfo = MWUtils.getAppInfo(webClient, url, accessToken);
        return MWUtils.getJsonNodeResponse(appInfo, "MerchantId");
    }

    public SynchronizedPriceList getSyncPriceList(String name, String currencyId, String dataAreaId) throws RuntimeException {
        return priceLists.getSyncPriceList(name, currencyId, dataAreaId);
    }

    public JsonNode createPriceList(String name, String description, String currencyId, String accessToken, String merchantId)
            throws RuntimeException, JsonProcessingException {
        String url = endpoints.getEndpointUrl("CREATE_PRICE_LIST", "MULTIVENDE").replace("{{merchant_id}}", merchantId);
        HttpHeaders headers = MWUtils.getHeaders(accessToken);
        MultiValueMap<String, String> bodyValues = new LinkedMultiValueMap<>();
        bodyValues.add("name", name);
        bodyValues.add("description", description);
        bodyValues.add("CurrencyId", currencyId);
        return MWUtils.validateResponse("An error occurred while creating the price list.",
                webClient.post()
                .uri(url)
                .headers(h -> h.addAll(headers))
                .body(BodyInserters.fromFormData(bodyValues))
                .retrieve()
                .bodyToMono(String.class)
                .block());
    }

    public SynchronizedPriceList savePriceListInfo(JsonNode response, String dataAreaId) throws RuntimeException {
        try {
            Company company = companies.getCompany(dataAreaId);
            ObjectMapper objMapPriceList = new ObjectMapper();
            SynchronizedPriceList priceList = objMapPriceList.readValue(response.toString(), SynchronizedPriceList.class);
            priceList.setCompany(company);
            priceLists.save(priceList);
            return priceList;
        } catch (RuntimeException | JsonProcessingException e){
            throw new RuntimeException("An error occurred while saving information of price list created.");
        }
    }

    public String getCurrencyId(String currencyCode, String accessToken, String merchantId) throws RuntimeException, JsonProcessingException {
        String response, url;
        JsonNode allCurrencies;
        HttpHeaders headers = MWUtils.getHeaders(accessToken);
        url = endpoints.getEndpointUrl("GET_CURRENCIES", "MULTIVENDE").replace("{{merchant_id}}", merchantId);
        allCurrencies = MWUtils.validateResponse("",
                webClient.get()
                .uri(url)
                .headers(h -> h.addAll(headers))
                .retrieve()
                .bodyToMono(String.class)
                .block());
        response = selectCurrencyId(currencyCode, allCurrencies.toString());
        return response;
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

    public String processPriceUpdate(String priceListId, HashMap<String, String> requestBody, int itemsPerCall, String accessToken, String dataAreaId)
            throws RuntimeException, JsonProcessingException {
        JSONObject response = new JSONObject();
        Company company = companies.getCompany(dataAreaId);
        String url = endpoints.getEndpointUrl("UPDATE_PRICE_BULK_SETs", "MULTIVENDE")
                .replace("{{product_price_list_id}}", priceListId);
        List<JSONArray> bodyValues = getItemPrices(requestBody, itemsPerCall, dataAreaId);
        JsonNode auxSyncPrices;
        for(JSONArray bodyValue: bodyValues){
            try {
                auxSyncPrices = MWUtils.validateResponse("", updatePrices(accessToken, url, bodyValue));
                updateSyncItemPriceDB(auxSyncPrices, priceListId, company);
                response.accumulate("UpdatePrices", new JSONArray(auxSyncPrices.toString()));
                System.out.println("Item Prices successfully updated.");
            }catch (RuntimeException e){
                response.accumulate("error", e.getMessage());
                System.err.println("Error while updating item prices.");
            }
        }
        return response.toString();
    }

    private List<JSONArray> getItemPrices(HashMap<String, String> requestBody, int itemsPerCall, String dataAreaId) throws RuntimeException {
        List<JSONArray> response = new ArrayList<>();
        JSONArray auxItemPrices = new JSONArray();
        float basePercentage = getBasePercentage(requestBody, dataAreaId), itemPercentage = 0, auxCost = 0;
        HashMap<String, String> itemCost = getItemMap(itemInventory.getItemCost());
        HashMap<String, String> itemCategory = getItemMap(itemInventory.getItemCategory());
        SynchronizedProducts[] syncProducts = this.syncProducts.getSynchronizedProducts();
        int countItemSyncProducts = 0, countItemArray = 0;
        for(SynchronizedProducts product : syncProducts){
            countItemSyncProducts++;
            try {
               if(itemCost.get(product.getInternalCode()) != null && itemCategory.get(product.getInternalCode()) != null ) {
                   countItemArray++;
                   itemPercentage = profitMargin.getProfitMargin("CATEGORY", itemCategory.get(product.getInternalCode()), requestBody.get("currencyCode"), dataAreaId).getPercentage().floatValue();
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
        ProfitMarginCategory[] marginCategories = this.marginCategories.getProfitMarginCategory(false, true);
        HashMap<String, String> bodyReqRelation = MWUtils.getMarginCategoriesBodyRelation();
        String auxCategoryName = "";
        float response = 0;
        for(ProfitMarginCategory category : marginCategories){
            auxCategoryName = category.getId().getName();
            response += profitMargin.getProfitMargin(auxCategoryName, requestBody.get(bodyReqRelation.get(auxCategoryName)),
                    requestBody.get("currencyCode"), dataAreaId).getPercentage().floatValue();
        }
        return response;
    }

    private String updatePrices(String accessToken, String url, JSONArray bodyValues) throws RuntimeException {
        HttpHeaders headers = MWUtils.getHeaders(accessToken);
        return webClient.post()
                .uri(url)
                .headers(h -> h.addAll(headers))
                .bodyValue(bodyValues.toString())
                .retrieve()
                .bodyToMono(String.class)
                .block();
    }

    private void updateSyncItemPriceDB(JsonNode jsonNodeResp, String priceListId, Company company) throws RuntimeException {
        JSONArray response = new JSONArray(jsonNodeResp.toString());
        ObjectMapper objMapSyncIte = new ObjectMapper();
        JSONObject auxSyncItemPriceObject;
        SynchronizedItemPrice auxSyncItemPrice;
        for(int it = 0; it < response.length(); it++){
            try {
                auxSyncItemPriceObject = response.getJSONObject(it);
                SynchronizedItemPrice syncItemPrice = objMapSyncIte.readValue(auxSyncItemPriceObject.toString(), SynchronizedItemPrice.class);
                auxSyncItemPrice = itemPrices.getSyncItemPrice(priceListId, syncItemPrice.getProductVersionId(), company.getId().getDataAreaId());
                syncItemPrice.setPriceListId(priceListId);
                syncItemPrice.setCompany(company);
                if(auxSyncItemPrice != null){
                    syncItemPrice.setId(auxSyncItemPrice.getId());
                }
                itemPrices.save(syncItemPrice);
            } catch (RuntimeException | JsonProcessingException e){
                System.err.println("Error while saving synchronized item price info.");
            }
        }
    }

}
