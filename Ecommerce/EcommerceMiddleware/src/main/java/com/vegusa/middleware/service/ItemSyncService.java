package com.vegusa.middleware.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.repository.*;
import com.vegusa.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.middleware.utils.MWUtils;
import org.json.JSONObject;
import org.springframework.core.env.Environment;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.util.LinkedMultiValueMap;
import org.springframework.util.MultiValueMap;
import org.springframework.web.reactive.function.BodyInserters;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.http.HttpHeaders;
import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.text.ParseException;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.HashMap;
import java.util.List;

@Service
public class ItemSyncService {
    private final ProductAttributeValueRepository attributeValueRepo;
    private final InterfaceItemsRepository iProductRepo;
    private final InterfaceHierarchyRepository iHierarchyRepo;
    private final ProductAttributeRepository attributeRepo;
    private final ProductAttributeHierarchyRepository attributeHierarchyRepo;
    private final SyncItemRepository syncItemRepo;
    private final SyncBrandRepository syncBrandRepo;
    private final SyncCategoryRepository syncCategoryRepo;
    private final CompanyRepository companyRepo;
    private final EndpointRepository endpointRepo;
    private final AuthTokenRepository authTokenRepo;
    private final WebClient webClient;
    private final Environment env;
    private EncryptDecryptInterface encryptDecryptInterface;
    private String algorithm;

    @Autowired
    public ItemSyncService(ProductAttributeValueRepository attributeValueRepo,
                           InterfaceItemsRepository iProductRepo,
                           InterfaceHierarchyRepository iHierarchyRepo,
                           ProductAttributeRepository attributeRepo,
                           ProductAttributeHierarchyRepository attributeHierarchyRepo,
                           SyncItemRepository syncItemRepo,
                           SyncBrandRepository syncBrandRepo,
                           SyncCategoryRepository syncCategoryRepo,
                           CompanyRepository companyRepo,
                           EndpointRepository endpointRepo,
                           AuthTokenRepository authTokenRepo,
                           WebClient webClient,
                           Environment env) {
        this.attributeValueRepo = attributeValueRepo;
        this.iProductRepo = iProductRepo;
        this.iHierarchyRepo = iHierarchyRepo;
        this.attributeRepo = attributeRepo;
        this.attributeHierarchyRepo = attributeHierarchyRepo;
        this.syncItemRepo = syncItemRepo;
        this.syncBrandRepo = syncBrandRepo;
        this.syncCategoryRepo = syncCategoryRepo;
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

    public Company getCompany(String dataAreaId) throws RuntimeException {
        return companyRepo.getCompany(dataAreaId);
    }

    public String processProductsUpdate(String authToken, String merchantId, String dataAreaId, Company company) throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException {
        JSONObject response = new JSONObject();
        String urlCreateProduct = endpointRepo.getEndpointUrl("CREATE_PRODUCT", env.getProperty("integration.company.name"))
                .replace("{{merchant_id}}", merchantId);
        String urlUpdateProduct = endpointRepo.getEndpointUrl("UPDATE_PRODUCT", env.getProperty("integration.company.name"));
        InterfaceItems auxIProduct = new InterfaceItems();
        List<String> itemIds = attributeValueRepo.getProdAttValueItemIds(dataAreaId);
        for(String itemId : itemIds){
            try {
                SyncItem syncItem = syncItemRepo.getSyncItem(itemId, dataAreaId);
                getProductToUpdate(auxIProduct, itemId, dataAreaId);
                SimpleDateFormat formatter = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss");
                String url = (syncItem == null) ? urlCreateProduct :
                        (formatter.parse(auxIProduct.getUpdatedAt().toString()).after(formatter.parse(syncItem.getUpdatedAtMv().toString())))
                                ? urlUpdateProduct.replace("{{product_id}}", syncItem.getIdMvd()) : "not synchronize";
                if(!url.equals("not synchronize")){
                    HashMap<String, String> idCatalogs = getCatalogs(authToken, merchantId, dataAreaId, auxIProduct.getBrand(), auxIProduct.getCategory());
                    String syncResponse = updateProduct(authToken, syncItem, url, auxIProduct, idCatalogs);
                    saveSyncProductsInfo(syncResponse, syncItem, company);
                    response.accumulate("ok", "The product " + itemId + " was updated successfully.");
                    System.out.println("The product " + itemId + " was updated successfully.");
                } else {
                    response.accumulate("synchronized", "The product " + itemId + " doesn't require to be updated.");
                    System.out.println("The product " + itemId + " doesn't require to be synchronized.");
                }
            } catch  (RuntimeException  | ParseException | JsonProcessingException e) {
                System.err.println("An error occurred while synchronizing the product " + itemId);
                response.accumulate("error", "An error occurred while updating the product " + itemId + " " + e.getMessage());
                if(e.getMessage().contains("401")){
                    authToken = MWUtils.getDecryptedAccessToken(authTokenRepo, encryptDecryptInterface,  env, algorithm,
                            "An error occurred while renewing unauthorized token.");
                }
            }
        }
        return response.toString();
    }

    private void getProductToUpdate(InterfaceItems iProduct, String itemId, String dataAreaId) throws RuntimeException {
        List<String> attributes = attributeRepo.getProductAttributeId(dataAreaId);
        HashMap<String, String> attrValueMap = MWUtils.getControlTableAttributes();
        List<String> auxHierarchies;
        String auxAttributeValue = "";
        for (String attribute : attributes){
            try {
                auxHierarchies = attributeHierarchyRepo.getAttributeHierarchy(attribute, dataAreaId);
                if (auxHierarchies == null) {
                    auxHierarchies = iHierarchyRepo.getInterfaceHierarchy(dataAreaId);
                }
                auxAttributeValue = getAttributeValue(auxHierarchies, attribute, itemId, dataAreaId);
                attrValueMap.put(attribute, auxAttributeValue);
            } catch (RuntimeException e){
                System.err.println("Error finding value for attribute " + attribute + " of Item Id " + itemId);
            }
        }
        setProductToUploadValues(iProduct, attrValueMap);
    }

    private String getAttributeValue(List<String> hierarchies, String attribute, String itemId, String dataAreaId) throws RuntimeException {
        String response = "";
        ProductAttributeValue auxAttributeValue;
        boolean foundValue = false, skipNull = Boolean.parseBoolean(iProductRepo.getSkipNull(itemId, dataAreaId));
        int countHierarchies = 0;
        do{
            auxAttributeValue = attributeValueRepo.getProductAttributeValue(attribute, itemId, hierarchies.get(countHierarchies), dataAreaId);
            if(auxAttributeValue != null){
                foundValue = true;
                response = auxAttributeValue.getValue();
            }
            countHierarchies++;
        } while(!foundValue && countHierarchies < hierarchies.size() && skipNull);
        return response;
    }

    private void setProductToUploadValues(InterfaceItems iProduct, HashMap<String, String> values){
        iProduct.setItemId(values.get("ITEM_ID"));
        iProduct.setProductName(values.get("PRODUCT_NAME"));
        iProduct.setPartNumber(values.get("PART_NUMBER"));
        iProduct.setShortDescription(values.get("SHORT_DESCRIPTION"));
        iProduct.setBrand(values.get("BRAND_ID"));
        iProduct.setCategory(values.get("CATEGORY_ID"));
        iProduct.setWeight(values.get("WEIGHT"));
        iProduct.setUnitOfMeasurement(values.get("UNIT_OF_MEASUREMENT"));
        iProduct.setAvailable(Double.parseDouble(values.get("AVAILABLE")));
        iProduct.setCost(Double.parseDouble(values.get("COST")));
    }

    private HashMap<String, String> getCatalogs(String accessToken, String merchantId, String dataAreaId, String brand, String category){
        try {
            HashMap<String, String> idCatalogs = new HashMap<>();
            SyncBrand syncBrandId = syncBrandRepo.getSyncBrand(brand, dataAreaId);
            SyncCategory syncCategory = syncCategoryRepo.getSyncCategory(category, dataAreaId);
            String brandId = syncBrandId != null ? syncBrandId.getIdEcom() : brand != null ?
                    createCatalogValue("BRANDS", accessToken, merchantId, endpointRepo.getEndpointUrl("POST_BRAND", env.getProperty("integration.company.name")), brand) : null;
            String categoryId = syncCategory != null ? syncCategory.getIdEcom() : category != null ?
                    createCatalogValue("CATEGORIES", accessToken, merchantId, endpointRepo.getEndpointUrl("CREATE_PRODUCT_CATEGORY", env.getProperty("integration.company.name")), category) : null;
            idCatalogs.put("brandId", brandId);
            idCatalogs.put("categoryId", categoryId);
            return idCatalogs;
        } catch (RuntimeException | JsonProcessingException e){
            throw new RuntimeException("An error occurred while obtaining the catalog IDs." + e.getMessage());
        }
    }

    private String createCatalogValue(String catalog, String accessToken, String merchantId, String url, String value) throws RuntimeException, JsonProcessingException {
        String catalogValue = "";
        HttpHeaders headers = new HttpHeaders();
        MultiValueMap<String, String> bodyValues = new LinkedMultiValueMap<>();
        ObjectMapper objMapSyncCatalogValue = new ObjectMapper();
        String createdCatalogValue;
        headers.add("Content-Type", "application/json");
        headers.add("Authorization", "Bearer " + accessToken);
        bodyValues.add("name", value);
        bodyValues.add("description", value);
        createdCatalogValue = webClient.post()
                .uri(url.replace("{{merchant_id}}", merchantId))
                .headers(h -> h.addAll(headers))
                .body(BodyInserters.fromFormData(bodyValues))
                .retrieve()
                .bodyToMono(String.class)
                .block();
        MWUtils.validateResponse("An error occurred while creating the catalog ID value to " + value + " in " + catalog, createdCatalogValue);
        switch(catalog) {
            case "BRANDS":
                SyncBrand vegEcomSynchronizedBrands = objMapSyncCatalogValue.readValue(createdCatalogValue, SyncBrand.class);
                vegEcomSynchronizedBrands.setVegCompany("MSB");
                syncBrandRepo.save(vegEcomSynchronizedBrands);
                catalogValue = vegEcomSynchronizedBrands.getIdEcom();
                break;
            case "CATEGORIES":
                SyncCategory vegEcomSynchronizedCategories = objMapSyncCatalogValue.readValue(createdCatalogValue, SyncCategory.class);
                vegEcomSynchronizedCategories.setVegCompany("MSB");
                syncCategoryRepo.save(vegEcomSynchronizedCategories);
                catalogValue = vegEcomSynchronizedCategories.getIdEcom();
                break;
            default:
                // code block
        }
        return catalogValue;
    }

    private String updateProduct(String accessToken, SyncItem synchronizedProduct, String url, InterfaceItems product, HashMap<String, String> idCatalogs) {
        try {
            HttpHeaders headers = new HttpHeaders();
            headers.add("Content-Type", "application/json");
            headers.add("Authorization", "Bearer " + accessToken);
            MultiValueMap<String, String> bodyValues = new LinkedMultiValueMap<>();
            bodyValues.add("name", product.getProductName());
            bodyValues.add("alias", product.getPartNumber());
            bodyValues.add("model", product.getPartNumber());
            bodyValues.add("description", product.getShortDescription());
            bodyValues.add("code", product.getPartNumber());
            bodyValues.add("internalCode", product.getItemId());
            bodyValues.add("InventoryTypeId", "791a6654-c5f2-11e6-aad6-2c56dc130c0d");
            if(idCatalogs.get("brandId") != null){ bodyValues.add("BrandId", idCatalogs.get("brandId")); }
            if(idCatalogs.get("categoryId") != null){ bodyValues.add("ProductCategoryId", idCatalogs.get("categoryId")); }
            if (synchronizedProduct == null) {
                return webClient.post()
                        .uri(url)
                        .headers(h -> h.addAll(headers))
                        .body(BodyInserters.fromFormData(bodyValues))
                        .retrieve()
                        .bodyToMono(String.class)
                        .block();
            } else {
                return webClient.put()
                        .uri(url)
                        .headers(h -> h.addAll(headers))
                        .body(BodyInserters.fromFormData(bodyValues))
                        .retrieve()
                        .bodyToMono(String.class)
                        .block();
            }
        } catch (RuntimeException e) {
            throw new RuntimeException("An error occurred while updating product " + product.getItemId() + " " + e.getMessage());
        }
    }

    private void saveSyncProductsInfo(String syncResponse, SyncItem synchronizedProduct, Company company) throws JsonProcessingException {
        try {
            ObjectMapper objMapSyncProducts = new ObjectMapper();
            SyncItem vegMvSynchronizedProduct = objMapSyncProducts.readValue(syncResponse, SyncItem.class);
            vegMvSynchronizedProduct.setCompany(company);
            vegMvSynchronizedProduct.setIntegrationCompany(env.getProperty("integration.company.name"));
            vegMvSynchronizedProduct.setVegSyncStatus("synchronized");
            if(synchronizedProduct != null) {
                vegMvSynchronizedProduct.setId(synchronizedProduct.getId());
            }
            syncItemRepo.save(vegMvSynchronizedProduct);
        } catch (RuntimeException | JsonProcessingException e) {
            updateMiddlewareSynchronizedProductsWithError(syncResponse, synchronizedProduct, company);
            throw new RuntimeException(e.getMessage());
        }
    }

    private void updateMiddlewareSynchronizedProductsWithError(String syncResponse, SyncItem synchronizedProduct, Company company) throws JsonProcessingException {
        try {
            SyncItem vegMvSynchronizedProduct = new SyncItem();
            if(synchronizedProduct != null){
                vegMvSynchronizedProduct.setId(synchronizedProduct.getId());
            }
            vegMvSynchronizedProduct.setIdMvd(MWUtils.getJsonNodeResponse(syncResponse, "_id"));
            vegMvSynchronizedProduct.setInternalCode(MWUtils.getJsonNodeResponse(syncResponse,"internalCode"));
            vegMvSynchronizedProduct.setCompany(company);
            vegMvSynchronizedProduct.setIntegrationCompany(env.getProperty("integration.company.name"));
            vegMvSynchronizedProduct.setVegSyncStatus("error");
            vegMvSynchronizedProduct.setUpdatedAtMv(new Date(0));
            syncItemRepo.save(vegMvSynchronizedProduct);
            System.err.println("The product " + MWUtils.getJsonNodeResponse(syncResponse,"internalCode") + " with error status was saved in Middleware table.");
        } catch (RuntimeException | JsonProcessingException e) {
            System.err.println("The product " + MWUtils.getJsonNodeResponse(syncResponse,"internalCode") + " with error status was NOT saved in Middleware table.");
        }
    }

    /*
    @Transactional(readOnly = false)
    public String processTVHAdditionalInfo() throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException,
            NoSuchAlgorithmException, BadPaddingException, InvalidKeyException {
        JSONObject response = new JSONObject();
        String accessToken = MWUtils.getDecryptedAccessToken(authTokenRepo, encryptDecryptInterface, env, algorithm);
        String urlUpdateProduct = endpointRepo.getEndpointUrl("UPDATE_PRODUCT", env.getProperty("integration.company.name"));
        VwVegEcommScrapedAdditionalInfo[] additionalInfo = vwVegEcommScrapedAdditionalInfoRepository.getAdditionalProductsInfo();
        for(int it = 0; it < additionalInfo.length; it++){
            try {
                HashMap<String, String> syncProduct = new HashMap<>();
                SyncItem synchronizedProduct = vegMvSynchronizedProductRepository
                        .getSyncItem(additionalInfo[it].getInternalProductId());
                String url = urlUpdateProduct.replace("{{product_id}}", additionalInfo[it].getIdMv());
                JsonNode jsonNodeSyncProducts = MWUtils
                        .validateResponse("", synchronizeProducts(accessToken, additionalInfo[it], url));
                updateMiddlewareSynchronizedProducts(jsonNodeSyncProducts, synchronizedProduct);
                syncProduct.put("ok", "The product " + additionalInfo[it].getInternalProductId() + " was synchronized successfully.");
                response.accumulate("ok", syncProduct);
                System.out.println("The product " + additionalInfo[it].getInternalProductId() + " was synchronized successfully.");
            } catch (RuntimeException | JsonProcessingException e) {
                System.err.println("Error when synchronizing the product " + additionalInfo[it].getInternalProductId());
                e.printStackTrace();
                HashMap<String, String> syncProductError = new HashMap<>();
                syncProductError.put("error", "Error when synchronizing the product " + additionalInfo[it].getInternalProductId());
                syncProductError.put("message", e.getMessage());
                response.accumulate("error", syncProductError);
                if(e.getMessage().contains("401")){
                    accessToken = MWUtils.getDecryptedAccessToken(authTokenRepo, encryptDecryptInterface, env, algorithm,
                            "Access token was not found in processProducts method.");
                }
            }
        }
        return response.toString();
    }

        private String synchronizeProducts(String accessToken, VwVegEcommScrapedAdditionalInfo product, String url) {
        try {
            HttpHeaders headers = new HttpHeaders();
            headers.add("Content-Type", "application/json");
            headers.add("Authorization", "Bearer " + accessToken);
            MultiValueMap<String, String> bodyValues = new LinkedMultiValueMap<>();
            bodyValues.add("name", product.getShortDescription());
            bodyValues.add("description", product.getShortDescription() + " "
                    + product.getWeight() + "-" + product.getUnitOfMeasure() + " "
                    + product.getCrossReference());
            bodyValues.add("shortDescription", product.getShortDescription());
            return webClient.put()
                    .uri(url)
                    .headers(h -> h.addAll(headers))
                    .body(BodyInserters.fromFormData(bodyValues))
                    .retrieve()
                    .bodyToMono(String.class)
                    .block();
        } catch (RuntimeException e) {
            return MWUtils.getSimpleJSONResponse("error", e.getMessage());
        }
    }
    */


}
