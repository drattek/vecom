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
import java.text.DecimalFormat;
import java.util.*;

@Service
public class ItemPriceSyncService {
    private final SyncPriceListRepository syncPriceListRepo;
    private final SyncItemPriceRepository itemPriceRepo;
    private final CategoryRepository categoryRepo;
    private final ChannelRepository channelRepo;
    private final PriceListParameterRepository priceListParameterRepo;
    private final PriceListRepository priceListRepo;
    private final ProductCategoryRepository productCategoryRepo;
    private final ItemInventLocationRepository itemInventoryRepo;
    private final SyncItemRepository syncItemRepo;
    private final CompanyRepository companyRepo;
    private final EndpointRepository endpointRepo;
    private final AuthTokenRepository authTokenRepo;
    private final SystemParameterRepository systemParameterRepo;
    private final WebClient webClient;
    private final Environment env;
    private EncryptDecryptInterface encryptDecryptInterface;
    private String algorithm;
    private PriceListParameter lowerLimitAddFixedCost;
    private PriceListParameter centralLimitAddFixedCost;
    private PriceListParameter upperLimitAddFixedCost;

    @Autowired
    private ItemPriceSyncService(SyncPriceListRepository syncPriceListRepo,
                                 SyncItemPriceRepository itemPriceRepo,
                                 CategoryRepository categoryRepo,
                                 ChannelRepository channelRepo,
                                 PriceListParameterRepository priceListParameterRepo,
                                 PriceListRepository priceListRepo,
                                 ProductCategoryRepository productCategoryRepo,
                                 ItemInventLocationRepository itemInventoryRepo,
                                 SyncItemRepository syncItemRepo,
                                 CompanyRepository companyRepo,
                                 EndpointRepository endpointRepo,
                                 AuthTokenRepository authTokenRepo,
                                 SystemParameterRepository systemParameterRepo,
                                 WebClient webClient,
                                 Environment env){
        this.syncPriceListRepo = syncPriceListRepo;
        this.itemPriceRepo = itemPriceRepo;
        this.categoryRepo = categoryRepo;
        this.channelRepo = channelRepo;
        this.priceListParameterRepo = priceListParameterRepo;
        this.priceListRepo = priceListRepo;
        this.productCategoryRepo = productCategoryRepo;
        this.itemInventoryRepo = itemInventoryRepo;
        this.syncItemRepo = syncItemRepo;
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

    public Company getCompany(String dataAreaId) throws RuntimeException{
        return companyRepo.getCompany(dataAreaId);
    }

    public int getProductsPerCall(String sysParameterName) throws RuntimeException {
        return systemParameterRepo.getSystemParameter(sysParameterName).getIntValue();
    }

    public SyncPriceList getSyncPriceList(String name, String currencyId, String dataAreaId) throws RuntimeException {
        return syncPriceListRepo.getSyncPriceList(name, currencyId, dataAreaId);
    }

    public void setAddFixedCostParameters(String lowerLimitName, String centralLimitName, String upperLimitName, String dataAreaId) {
       try{
           this.lowerLimitAddFixedCost = priceListParameterRepo.getPriceListParameter(lowerLimitName, dataAreaId);
           this.centralLimitAddFixedCost = priceListParameterRepo.getPriceListParameter(centralLimitName, dataAreaId);
           this.upperLimitAddFixedCost = priceListParameterRepo.getPriceListParameter(upperLimitName, dataAreaId);
       } catch (RuntimeException e){
           throw new RuntimeException("An error occurred while obtaining the parameters for calculate the fixed cost: " + e.getMessage());
       }
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

    public SyncPriceList savePriceListInfo(String response, Company company) throws RuntimeException {
        try {
            ObjectMapper objMapPriceList = new ObjectMapper();
            SyncPriceList priceList = objMapPriceList.readValue(response, SyncPriceList.class);
            priceList.setCompany(company);
            syncPriceListRepo.save(priceList);
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

    public String processPriceListUpdate(SyncPriceList syncPriceList, String priceListName, String channel, String currencyCode, int itemsPerCall, String accessToken)
            throws RuntimeException, JsonProcessingException {
        JSONObject response = new JSONObject();
        Company company = syncPriceList.getCompany();
        String url = endpointRepo.getEndpointUrl("UPDATE_PRICE_BULK_SET", env.getProperty("integration.company.name"))
                .replace("{{product_price_list_id}}", syncPriceList.getResponseId());
        List<JSONArray> bodyValues = getItemPrices(priceListName, channel, currencyCode, itemsPerCall, company.getId().getDataAreaId());
        for(JSONArray bodyValue: bodyValues){
            try {
                String updatedPrices = updatePrices(accessToken, url, bodyValue);
                saveSyncPriceListInfo(updatedPrices, syncPriceList.getResponseId(), company);
                response.accumulate("updated", new JSONArray(updatedPrices));
                System.out.println("A part of the price list was successfully updated.");
            }catch (RuntimeException e){
                response.accumulate("error", e.getMessage());
                System.err.println("An error occurred while updating a part of price list.");
            }
        }
        System.out.println("Price Lists Update Ends.");
        return response.toString();
    }

    private List<JSONArray> getItemPrices(String priceListName, String channel, String currencyCode, int itemsPerCall, String dataAreaId) throws RuntimeException {
        List<JSONArray> response = new ArrayList<>();
        JSONArray itemPriceArray = new JSONArray();
        PriceListParameter shippingCostParameter = priceListParameterRepo.getPriceListParameter("SHIPPING_COST", dataAreaId);
        float basePercentage = getBasePercentage(priceListName, channel, currencyCode, dataAreaId),
                shippingCost = shippingCostParameter != null ? shippingCostParameter.getDecValue().floatValue() : 0,
                auxItemPercentage = 0, auxCost = 0, finalCost = 0;
        HashMap<String, String> itemCostMap = getItemMap(itemInventoryRepo.getItemCost());
        HashMap<String, String> itemCategoryMap = getItemMap(productCategoryRepo.getItemCategory(dataAreaId));
        SyncItem[] syncItems = this.syncItemRepo.getSyncItem();
        DecimalFormat costFmt = new DecimalFormat("0.00");
        int countSyncItems = 0, countItemArray = 0;
        for(SyncItem product : syncItems){
            countSyncItems++;
            try {
                if(itemCostMap.get(product.getInternalCode()) == null && itemCategoryMap.get(product.getInternalCode()) == null){
                    System.out.println("Entra a esta parte!");
                }
               if(itemCostMap.get(product.getInternalCode()) != null && itemCategoryMap.get(product.getInternalCode()) != null
               && product.getDefaultVersionId() != null) {
                    countItemArray++;
                    auxItemPercentage = getCategoryPercentage(itemCategoryMap.get(product.getInternalCode()));
                    auxCost = (((basePercentage + auxItemPercentage) / 100) + 1) * Float.parseFloat(itemCostMap.get(product.getInternalCode()));
                    finalCost = getAdditionalFixedCost(auxCost) + shippingCost;
                    JSONObject itemPrice = new JSONObject();
                    itemPrice.put("ProductVersionId", product.getDefaultVersionId());
                    itemPrice.put("gross", costFmt.format(finalCost));
                    itemPrice.put("priceWithDiscount", costFmt.format(finalCost));
                    itemPrice.put("tax", 16);
                    itemPrice.put("net", costFmt.format(finalCost));
                    itemPriceArray.put(itemPrice);
                   if (countItemArray % itemsPerCall == 0) {
                       response.add(itemPriceArray);
                       itemPriceArray = new JSONArray();
                       countItemArray = 0;
                   }
                }
                if(countSyncItems == syncItems.length && !itemPriceArray.isEmpty()){
                    response.add(itemPriceArray);
                }
            } catch (RuntimeException e){
                System.err.println("An error occurred obtaining item price of " + product.getInternalCode() + " " + e.getMessage());
            }
        }
        return response;
    }

    private HashMap<String, String> getItemMap(List<Object[]> itemValues) throws RuntimeException {
        HashMap<String, String> response = new HashMap<>();
        for(Object[] itemValue: itemValues){
            try {
                if(itemValue[0] != null && itemValue[1] != null) {
                    response.put(itemValue[0].toString(), itemValue[1].toString());
                }
            }catch (RuntimeException e){
                System.err.println("An error occurred while saving item value map.");
            }
        }
        return response;
    }

    private float getBasePercentage(String priceListName, String channel, String currencyCode, String dataAreaId) {
        try {
            float priceListPercentage = priceListRepo.getPriceList(priceListName, currencyCode, dataAreaId).getPercentage().floatValue();
            float channelPercentage = channelRepo.getChannel(channel, currencyCode, dataAreaId).getPercentage().floatValue();
            return priceListPercentage + channelPercentage;
        } catch (RuntimeException e){
            throw new RuntimeException("An error occurred while obtaining the base percentage: " + e);
        }
    }

    private float getCategoryPercentage(String recId) throws RuntimeException {
        float percentage = 0;
        int level = 0;
        Category category;
        boolean hasPercentage = false;
        Long auxParentCategory = Long.parseLong(recId);
        do{
            category = categoryRepo.getCategory(auxParentCategory);
            level = category.getLevel();
            auxParentCategory = category.getParentCategory();
            percentage = category.getPercentage().floatValue();
            if(percentage != 0){
                hasPercentage = true;
            }
        }while (auxParentCategory != 0 && level != 1 && !hasPercentage);
        return percentage;
    }

    private float getAdditionalFixedCost(float cost) {
        float response = 0;
        if(this.lowerLimitAddFixedCost != null && this.centralLimitAddFixedCost != null && this.upperLimitAddFixedCost != null) {
            if(cost < this.lowerLimitAddFixedCost.getIntValue()){
                response = cost + this.lowerLimitAddFixedCost.getIntValue() - cost + this.lowerLimitAddFixedCost.getDecValue().floatValue();
            } else if (cost < this.centralLimitAddFixedCost.getIntValue()) {
                response = cost + this.lowerLimitAddFixedCost.getDecValue().floatValue();
            } else if (cost < this.upperLimitAddFixedCost.getIntValue()) {
                response = cost + this.centralLimitAddFixedCost.getDecValue().floatValue();
            } else {
                response = cost + this.upperLimitAddFixedCost.getDecValue().floatValue();
            }
        }
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
            throw new RuntimeException("An error occurred while updating price list: " + e.getMessage());
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
             //   System.err.println("Error while saving synchronized item price info.");
            }
        }
    }

}
