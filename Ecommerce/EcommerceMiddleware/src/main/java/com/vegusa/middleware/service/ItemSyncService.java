package com.vegusa.middleware.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.repository.*;
import com.vegusa.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.middleware.utils.MWUtils;
import jakarta.persistence.EntityManager;
import org.json.JSONObject;
import org.springframework.core.env.Environment;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
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
    //Middleware - Repository
    private final EndpointRepository endpointRepo;
    private final AuthTokenRepository tokenInfoRepository;
    private final SyncProductsRepository vegMvSynchronizedProductRepository;
    private final VwVegEcommScrapedAdditionalInfoRepository vwVegEcommScrapedAdditionalInfoRepository;
    private final VegEcomSynchronizedBrandsRepository vegEcomSynchronizedBrandsRepository;
    private final VegEcomSynchronizedCategoriesRepository vegEcomSynchronizedCategoriesRepository;
    private final InterfaceHierarchyRepository interfaceHierarchies;
    private final ProductAttributeRepository attributes;
    private final ProductAttributeHierarchyRepository attributeHierarchies;
    private final ProductAttributeValueRepository attributeValues;
    private final InterfaceProductRepository productRepository;
    //Global
    private final WebClient webClient;
    private final Environment env;
    private final EntityManager entityManager;
    private EncryptDecryptInterface encryptDecryptInterface;
    private String algorithm;

    @Autowired
    public ItemSyncService(EndpointRepository endpointRepo,
                           AuthTokenRepository tokenInfoRepository,
                           SyncProductsRepository vegMvSynchronizedProductRepository,
                           VwVegEcommScrapedAdditionalInfoRepository vwVegEcommScrapedAdditionalInfoRepository,
                           VegEcomSynchronizedBrandsRepository vegEcomSynchronizedBrandsRepository,
                           VegEcomSynchronizedCategoriesRepository vegEcomSynchronizedCategoriesRepository,
                           InterfaceHierarchyRepository interfaceHierarchies,
                           ProductAttributeRepository attributes,
                           ProductAttributeHierarchyRepository attributeHierarchies,
                           ProductAttributeValueRepository attributeValues,
                           InterfaceProductRepository productRepository,
                           WebClient webClient,
                           Environment env,
                           EntityManager entityManager) {
        this.endpointRepo = endpointRepo;
        this.tokenInfoRepository = tokenInfoRepository;
        this.vegMvSynchronizedProductRepository = vegMvSynchronizedProductRepository;
        this.vwVegEcommScrapedAdditionalInfoRepository = vwVegEcommScrapedAdditionalInfoRepository;
        this.vegEcomSynchronizedBrandsRepository = vegEcomSynchronizedBrandsRepository;
        this.interfaceHierarchies = interfaceHierarchies;
        this.attributes = attributes;
        this.attributeHierarchies = attributeHierarchies;
        this.attributeValues = attributeValues;
        this.productRepository = productRepository;
        this.vegEcomSynchronizedCategoriesRepository = vegEcomSynchronizedCategoriesRepository;
        this.webClient = webClient;
        this.env = env;
        this.entityManager = entityManager;
    }

    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface, String algorithm) {
        this.encryptDecryptInterface = encryptDecryptInterface;
        this.algorithm = algorithm;
    }

    private String getMerchantId(String accessToken) throws RuntimeException, JsonProcessingException {
        String url = endpointRepo.getEndpointUrl("GET_APP_INFORMATION", env.getProperty("integration.company.name"));
        String appInfo = MWUtils.getAppInfo(webClient, url, accessToken);
        return MWUtils.getJsonNodeResponse(appInfo, "MerchantId");
    }

    public String processAndUploadProducts(String dataAreaId) throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException,
            IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException {
        JSONObject response = new JSONObject();
        String accessToken = MWUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface, env, algorithm);
        String merchantId = getMerchantId(accessToken);
        String urlCreateProduct = endpointRepo.getEndpointUrl("CREATE_PRODUCT", env.getProperty("integration.company.name"))
                .replace("{{merchant_id}}", merchantId);
        String urlUpdateProduct = endpointRepo.getEndpointUrl("UPDATE_PRODUCT", env.getProperty("integration.company.name"));
        InterfaceProduct auxIProduct = new InterfaceProduct();
        List<String> itemIds = attributeValues.getItemIdList(dataAreaId);
        for(String itemId : itemIds){
            try {
                HashMap<String, String> syncProduct = new HashMap<>();
                SynchronizedProducts synchronizedProduct = vegMvSynchronizedProductRepository.getSynchronizedProductById(itemId);
                getProductToUpload(auxIProduct, itemId, dataAreaId);
                SimpleDateFormat formatter = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss");
                String url = (synchronizedProduct == null) ? urlCreateProduct :
                        (formatter.parse(auxIProduct.getUpdatedAt().toString()).after(formatter.parse(synchronizedProduct.getUpdatedAtMv().toString())))
                                ? urlUpdateProduct.replace("{{product_id}}", synchronizedProduct.getIdMvd()) : "not synchronize";
                if(!url.equals("not synchronize")){
                    HashMap<String, String> idCatalogs = getCatalogs(accessToken, merchantId, auxIProduct.getBrand(), auxIProduct.getCategory());
                    JsonNode jsonNodeSyncProducts = MWUtils
                            .validateResponse("", synchronizeProducts(accessToken, synchronizedProduct, url, auxIProduct, idCatalogs));
                    updateMiddlewareSynchronizedProducts(jsonNodeSyncProducts, synchronizedProduct);
                    syncProduct.put("ok", "The product " + itemId + " was synchronized successfully.");
                    response.accumulate("ok", syncProduct);
                    System.out.println("The product " + itemId + " was synchronized successfully.");
                } else {
                    syncProduct.put("ok", "The product " + itemId + " doesn´t require to be synchronized.");
                    response.accumulate("synchronized", syncProduct);
                    System.out.println("The product " + itemId + " doesn´t require to be synchronized.");
                }
            } catch  (RuntimeException  | ParseException /*| JsonProcessingException*/ e) {
                System.err.println("Error when synchronizing the product " + itemId);
                e.printStackTrace();
                HashMap<String, String> syncProductError = new HashMap<>();
                syncProductError.put("error", "Error when synchronizing the product " + itemId);
                syncProductError.put("message", e.getMessage());
                response.accumulate("error", syncProductError);
                if(e.getMessage().contains("401")){
                    accessToken = MWUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface,  env, algorithm,
                            "Access token was not found in processProducts method.");
                }
            }
        }
        return response.toString();
    }

    private void getProductToUpload(InterfaceProduct iProduct, String itemId, String dataAreaId) throws RuntimeException {
        List<String> attributes = this.attributes.getProductAttribute(dataAreaId);
        HashMap<String, String> attrValueMap = MWUtils.getControlTableAttributes();
        List<String> auxHierarchies;
        String auxAttributeValue = "";
        for (String attribute : attributes){
            try {
                auxHierarchies = attributeHierarchies.getAttributeHierarchy(attribute, dataAreaId);
                if (auxHierarchies == null) {
                    auxHierarchies = interfaceHierarchies.getInterfaceHierarchy(dataAreaId);
                }
                auxAttributeValue = getAttributeValue(auxHierarchies, attribute, itemId, dataAreaId);
                attrValueMap.put(attribute, auxAttributeValue);
            } catch (RuntimeException e){
                System.err.println("Error finding value for attribute " + attribute + " of Item Id " + itemId);
            }
        }
        setProductToUploadValues(iProduct, attrValueMap);
    }

    private String getAttributeValue(List<String> hierarchies, String attribute, String itemId, String dataAreaId) throws RuntimeException{
        String response = "";
        ProductAttributeValue auxAttributeValue;
        boolean foundValue = false, skipNull = Boolean.parseBoolean(productRepository.getSkipNull(itemId, dataAreaId));
        int countHierarchies = 0;
        do{
            auxAttributeValue = attributeValues.getProductAttributeValue(attribute, itemId, hierarchies.get(countHierarchies), dataAreaId);
            if(auxAttributeValue != null){
                foundValue = true;
                response = auxAttributeValue.getValue();
            }
            countHierarchies++;
        } while(!foundValue && countHierarchies < hierarchies.size() && skipNull);
        return response;
    }

    private void setProductToUploadValues(InterfaceProduct iProduct, HashMap<String, String> values){
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

    private HashMap<String, String> getCatalogs(String accessToken, String merchantId, String brand, String category){
        try {
            HashMap<String, String> idCatalogs = new HashMap<>();
            VegEcomSynchronizedBrands syncBrandId = vegEcomSynchronizedBrandsRepository.getSynchronizedBrand(brand, "MSB");
            VegEcomSynchronizedCategories syncCategory = vegEcomSynchronizedCategoriesRepository.getSynchronizedCategory(category, "MSB");
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
                VegEcomSynchronizedBrands vegEcomSynchronizedBrands = objMapSyncCatalogValue.readValue(createdCatalogValue, VegEcomSynchronizedBrands.class);
                vegEcomSynchronizedBrands.setVegCompany("MSB");
                vegEcomSynchronizedBrandsRepository.save(vegEcomSynchronizedBrands);
                catalogValue = vegEcomSynchronizedBrands.getIdEcom();
                break;
            case "CATEGORIES":
                VegEcomSynchronizedCategories vegEcomSynchronizedCategories = objMapSyncCatalogValue.readValue(createdCatalogValue, VegEcomSynchronizedCategories.class);
                vegEcomSynchronizedCategories.setVegCompany("MSB");
                vegEcomSynchronizedCategoriesRepository.save(vegEcomSynchronizedCategories);
                catalogValue = vegEcomSynchronizedCategories.getIdEcom();
                break;
            default:
                // code block
        }
        return catalogValue;
    }

    private String synchronizeProducts(String accessToken, SynchronizedProducts synchronizedProduct, String url, InterfaceProduct product, HashMap<String, String> idCatalogs) {
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
            return MWUtils.getSimpleJSONResponse("error", e.getMessage());
        }
    }

    private void updateMiddlewareSynchronizedProducts(JsonNode jsonNodeResp, SynchronizedProducts synchronizedProduct) {
        try {
            ObjectMapper objMapSyncProducts = new ObjectMapper();
            SynchronizedProducts vegMvSynchronizedProduct = objMapSyncProducts.readValue(jsonNodeResp.toString(), SynchronizedProducts.class);
            vegMvSynchronizedProduct.setVegBusinessUnit("MSB");
            vegMvSynchronizedProduct.setIntegrationCompany(env.getProperty("integration.company.name"));
            vegMvSynchronizedProduct.setVegSyncStatus("synchronized");
            if(synchronizedProduct != null) {
                vegMvSynchronizedProduct.setId(synchronizedProduct.getId());
            }
            vegMvSynchronizedProductRepository.save(vegMvSynchronizedProduct);
        } catch (RuntimeException | JsonProcessingException e) {
            updateMiddlewareSynchronizedProductsWithError(jsonNodeResp, synchronizedProduct);
            throw new RuntimeException(e.getMessage());
        }
    }

    private void updateMiddlewareSynchronizedProductsWithError(JsonNode jsonNodeResp, SynchronizedProducts synchronizedProduct){
        try {
            SynchronizedProducts vegMvSynchronizedProduct = new SynchronizedProducts();
            if(synchronizedProduct != null){
                vegMvSynchronizedProduct.setId(synchronizedProduct.getId());
            }
            vegMvSynchronizedProduct.setIdMvd(jsonNodeResp.get("_id").asText());
            vegMvSynchronizedProduct.setInternalCode(jsonNodeResp.get("internalCode").asText());
            vegMvSynchronizedProduct.setVegBusinessUnit("MSB");
            vegMvSynchronizedProduct.setIntegrationCompany(env.getProperty("integration.company.name"));
            vegMvSynchronizedProduct.setVegSyncStatus("error");
            vegMvSynchronizedProduct.setUpdatedAtMv(new Date(0));
            vegMvSynchronizedProductRepository.save(vegMvSynchronizedProduct);
            System.err.println("The product " + jsonNodeResp.get("internalCode").asText() + " with error status was saved in Middleware table.");
        } catch (RuntimeException e) {
            System.err.println("The product " + jsonNodeResp.get("internalCode").asText() + " with error status was NOT saved in Middleware table.");
        }
    }

    @Transactional(readOnly = false)
    public String processTVHAdditionalInfo() throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException,
            NoSuchAlgorithmException, BadPaddingException, InvalidKeyException {
        JSONObject response = new JSONObject();
        String accessToken = MWUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface, env, algorithm);
        String urlUpdateProduct = endpointRepo.getEndpointUrl("UPDATE_PRODUCT", env.getProperty("integration.company.name"));
        VwVegEcommScrapedAdditionalInfo[] additionalInfo = vwVegEcommScrapedAdditionalInfoRepository.getAdditionalProductsInfo();
        for(int it = 0; it < additionalInfo.length; it++){
            try {
                HashMap<String, String> syncProduct = new HashMap<>();
                SynchronizedProducts synchronizedProduct = vegMvSynchronizedProductRepository
                        .getSynchronizedProductById(additionalInfo[it].getInternalProductId());
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
                    accessToken = MWUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface, env, algorithm,
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




}
