package com.vegusa.middleware.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.repository.*;
import com.vegusa.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.middleware.utils.MWUtils;
import com.vegusa.oauth2_0.service.AuthService;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.core.env.Environment;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.scheduling.annotation.Scheduled;
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
import java.text.DecimalFormat;
import java.text.ParseException;
import java.text.SimpleDateFormat;
import java.util.*;

@Service
public class ItemSyncService {
    private final ProductAttributeValueRepository attributeValueRepo;
    private final InterfaceItemsRepository iProductRepo;
    private final InterfaceHierarchyRepository iHierarchyRepo;
    private final ProductAttributeRepository attributeRepo;
    private final ProductAttributeHierarchyRepository attributeHierarchyRepo;
    private final ProductCategoryRepository productCategoryRepo;
    private final CategoryRepository categoryRepo;
    private final SyncItemRepository syncItemRepo;
    private final SyncBrandRepository syncBrandRepo;
    private final SyncCategoryRepository syncCategoryRepo;
    private final SyncTagRepository syncTagRepo;
    private final CompanyRepository companyRepo;
    private final EndpointRepository endpointRepo;
    private final AuthTokenRepository authTokenRepo;
    private final AuthService authService;
    private AuthToken authToken;
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
                           ProductCategoryRepository productCategoryRepo,
                           CategoryRepository categoryRepo,
                           SyncItemRepository syncItemRepo,
                           SyncBrandRepository syncBrandRepo,
                           SyncCategoryRepository syncCategoryRepo,
                           SyncTagRepository syncTagRepo,
                           CompanyRepository companyRepo,
                           EndpointRepository endpointRepo,
                           AuthTokenRepository authTokenRepo,
                           AuthService authService,
                           WebClient webClient,
                           Environment env) {
        this.attributeValueRepo = attributeValueRepo;
        this.iProductRepo = iProductRepo;
        this.iHierarchyRepo = iHierarchyRepo;
        this.attributeRepo = attributeRepo;
        this.attributeHierarchyRepo = attributeHierarchyRepo;
        this.productCategoryRepo = productCategoryRepo;
        this.categoryRepo = categoryRepo;
        this.syncItemRepo = syncItemRepo;
        this.syncBrandRepo = syncBrandRepo;
        this.syncCategoryRepo = syncCategoryRepo;
        this.syncTagRepo = syncTagRepo;
        this.companyRepo = companyRepo;
        this.endpointRepo = endpointRepo;
        this.authTokenRepo = authTokenRepo;
        this.authService = authService;
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
                    (formatter.parse(auxIProduct.getUpdatedAt().toString()).after(formatter.parse(syncItem.getUpdatedAt().toString())))
                                ? urlUpdateProduct.replace("{{product_id}}", syncItem.getResponseId()) : "not synchronize";
                if(!url.equals("not synchronize")){
                    String brandId = getBrandID(auxIProduct.getBrand(), authToken, merchantId, dataAreaId, company);
                    ArrayList<List<String>> categoryIDs = getCategoryIDs(itemId, authToken, merchantId, dataAreaId, company);
                    String syncResponse = updateProduct(authToken, syncItem, url, auxIProduct, brandId, categoryIDs);
                    saveSyncProductsInfo(syncResponse, syncItem, company);
                    response.accumulate("ok", "The product " + itemId + " was updated successfully.");
                    System.out.println("The product " + itemId + " was updated successfully.");
                } else {
                    response.accumulate("synchronized", "The product " + itemId + " doesn't require to be updated.");
                    System.out.println("The product " + itemId + " doesn't require to be synchronized.");
                }
            } catch  (RuntimeException | ParseException | JsonProcessingException e) {
                System.err.println("An error occurred while synchronizing the product " + itemId + " " +  e.getMessage());
                response.accumulate("error", "An error occurred while updating the product " + itemId + " " + e.getMessage());
                if(e.getMessage().contains("401") || e.getMessage().contains("404")){
                    authToken = MWUtils.getDecryptedAccessToken(authService.getAuthToken(), encryptDecryptInterface, algorithm,
                "An error occurred while renewing unauthorized token.");
                }
            }
        }
        return response.toString();
    }

    private void getProductToUpdate(InterfaceItems iProduct, String itemId, String dataAreaId) throws RuntimeException {
        List<String> attributes = attributeRepo.getProductAttributeId(dataAreaId);
        HashMap<String, String> attrValueMap = MWUtils.getControlTableAttributes();
        Date updatedAt = attributeValueRepo.getUpdatedDate(itemId, dataAreaId);
        List<String> auxHierarchies;
        String auxAttributeValue = "";
        for (String attribute : attributes){
            try {
                auxHierarchies = attributeHierarchyRepo.getAttributeHierarchy(attribute, dataAreaId);
                if (auxHierarchies.isEmpty()) {
                    auxHierarchies = iHierarchyRepo.getInterfaceHierarchy(dataAreaId);
                }
                auxAttributeValue = getAttributeValue(auxHierarchies, attribute, itemId, dataAreaId);
                attrValueMap.put(attribute, auxAttributeValue);
            } catch (RuntimeException e){
                System.err.println("Error finding value for attribute " + attribute + " of Item Id " + itemId);
            }
        }
        setProductToUploadValues(iProduct, attrValueMap, updatedAt);
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

    private void setProductToUploadValues(InterfaceItems iProduct, HashMap<String, String> values, Date updatedAt) throws RuntimeException {
        iProduct.setItemId(values.get("ITEM_ID"));
        iProduct.setProductName(values.get("PRODUCT_NAME"));
        iProduct.setPartNumber(values.get("PART_NUMBER"));
        iProduct.setShortDescription(values.get("SHORT_DESCRIPTION"));
        iProduct.setBrand(values.get("BRAND_ID"));
        iProduct.setCategory(values.get("CATEGORY_ID"));
        iProduct.setWeight(values.get("WEIGHT"));
        iProduct.setUnitOfMeasurement(values.get("UNIT_OF_MEASUREMENT"));
        if(values.get("AVAILABLE") != null){
            iProduct.setAvailable(Double.parseDouble(values.get("AVAILABLE")));
        }
        if(values.get("COST") != null){
            iProduct.setCost(Double.parseDouble(values.get("COST")));
        }
        iProduct.setLength(values.get("LENGTH"));
        iProduct.setHeight(values.get("HEIGHT"));
        iProduct.setWidth(values.get("WIDTH"));
        iProduct.setCrossReferences(values.get("CROSS_REFERENCES"));
        iProduct.setUpdatedAt(updatedAt);
    }

    private String getBrandID(String brand, String accessToken, String merchantId, String dataAreaId, Company company){
        try {
            SyncBrand syncBrandId = syncBrandRepo.getSyncBrand(brand, dataAreaId);
            return syncBrandId != null ? syncBrandId.getResponseId() : brand != null ?
                    createCatalogValue("BRANDS", accessToken, merchantId, endpointRepo.getEndpointUrl("POST_BRAND", env.getProperty("integration.company.name")), brand, company) : null;
        } catch (RuntimeException | JsonProcessingException e){
            throw new RuntimeException("An error occurred while obtaining Brand ID." + e.getMessage());
        }
    }

    private ArrayList<List<String>> getCategoryIDs(String itemId, String accessToken, String merchantId, String dataAreaId, Company company){
        try {
            ArrayList<List<String>> response = new ArrayList<List<String>>();
            List<String> categories = new ArrayList<>() , tags = new ArrayList<>();
            ProductCategory productCategory = productCategoryRepo.getProductCategory(itemId, dataAreaId);
            String categoryName = "", categoryId = "", tagId = "";
            Long auxCategoryRefRecId = productCategory != null ? productCategory.getCategory().getId().getRecId() : null;
            Category category;
            SyncCategory syncCategory;
            SyncTag syncTag;
            if(productCategory != null){
                do{
                    try {
                        category = categoryRepo.getCategory(auxCategoryRefRecId);
                        categoryName = category.getId().getName();
                        syncCategory = syncCategoryRepo.getSyncCategory(categoryName, dataAreaId);
                        categoryId = syncCategory != null ? syncCategory.getResponseId() : categoryName != null ?
                                createCatalogValue("CATEGORIES", accessToken, merchantId, endpointRepo.getEndpointUrl("CREATE_PRODUCT_CATEGORY", env.getProperty("integration.company.name")), categoryName, company) : null;
                        if(categoryId != null){
                            categories.add(categoryId);
                        }
                        syncTag = syncTagRepo.getSyncTag(categoryName, dataAreaId);
                        tagId = syncTag != null ? syncTag.getResponseId() : categoryName != null ?
                                createCatalogValue("TAGS", accessToken, merchantId, endpointRepo.getEndpointUrl("CREATE_TAGS", env.getProperty("integration.company.name")), categoryName, company) : null;;
                        if(tagId != null){
                            tags.add(tagId);
                        }
                        auxCategoryRefRecId = category.getParentCategory();
                    } catch (RuntimeException e) {
                        System.out.println("An error occurred while obtaining the Category ID. " + e.getMessage());
                    }
                } while(auxCategoryRefRecId != 0);
            }
            response.add(categories);
            response.add(tags);
            return response;
        } catch (RuntimeException | JsonProcessingException e){
            throw new RuntimeException("An error occurred while obtaining Category IDs. " + e.getMessage());
        }
    }

    private String createCatalogValue(String catalog, String accessToken, String merchantId, String url, String value, Company company) throws RuntimeException, JsonProcessingException {
        String catalogValue = "";
        HttpHeaders headers = MWUtils.getHeaders(accessToken);
        MultiValueMap<String, String> bodyValues = new LinkedMultiValueMap<>();
        ObjectMapper objMapSyncCatalogValue = new ObjectMapper();
        String createdCatalogValue;
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
                SyncBrand syncBrand = objMapSyncCatalogValue.readValue(createdCatalogValue, SyncBrand.class);
                syncBrand.setCompany(company);
                syncBrandRepo.save(syncBrand);
                catalogValue = syncBrand.getResponseId();
                break;
            case "CATEGORIES":
                SyncCategory syncCategory = objMapSyncCatalogValue.readValue(createdCatalogValue, SyncCategory.class);
                syncCategory.setCompany(company);
                syncCategoryRepo.save(syncCategory);
                catalogValue = syncCategory.getResponseId();
                break;
            case "TAGS":
                SyncTag syncTag = objMapSyncCatalogValue.readValue(createdCatalogValue, SyncTag.class);
                syncTag.setCompany(company);
                syncTagRepo.save(syncTag);
                catalogValue = syncTag.getResponseId();
            default:
                // code block
        }
        return catalogValue;
    }

    private String updateProduct(String accessToken, SyncItem syncItem, String url, InterfaceItems product, String brandId, ArrayList<List<String>> categoryIDs) {
        try {
            HttpHeaders headers = MWUtils.getHeaders(accessToken);
            List<String> categories = categoryIDs.getFirst(), tags = categoryIDs.getLast();
            JSONObject bodyValues = getUpdateProductBodyValues(syncItem, product, brandId, categories, tags);
            if (syncItem == null) {
                return webClient.post()
                        .uri(url)
                        .headers(h -> h.addAll(headers))
                        .bodyValue(bodyValues.toString())
                        .retrieve()
                        .bodyToMono(String.class)
                        .block();
            } else {
                return webClient.put()
                        .uri(url)
                        .headers(h -> h.addAll(headers))
                        .bodyValue(bodyValues.toString())
                        .retrieve()
                        .bodyToMono(String.class)
                        .block();
            }
        } catch (RuntimeException e) {
            throw new RuntimeException("An error occurred while updating product " + product.getItemId() + " " + e.getMessage());
        }
    }

    private static JSONObject getUpdateProductBodyValues(SyncItem syncItem, InterfaceItems product, String brandId, List<String> categories, List<String> tags) throws RuntimeException {
        JSONObject bodyValues = new JSONObject();
        JSONArray otherCategoriesArray = new JSONArray();
        JSONArray tagsArray = new JSONArray();
        String auxShortDescription = getShortDescription(product.getShortDescription(), product.getProductName()),
                name = getName(product, product.getShortDescription()),
                shortDescription = getName(product, auxShortDescription),
                description = "-- TIENDA VEGUSA MAQUINARIA, DISTRUIBIDOR AUTORIZADO UNICARRIERS, BOBCAT, JLG, FLEXI. -- " + shortDescription + ". " + getCrossReferences(product.getCrossReferences());
        JSONArray productVersionsArray = new JSONArray();
        bodyValues.put("name", name);
        bodyValues.put("alias", product.getProductName());
        bodyValues.put("model", product.getPartNumber());
        bodyValues.put("description", description);
        bodyValues.put("shortDescription", shortDescription);
        bodyValues.put("code", product.getPartNumber());
        bodyValues.put("internalCode", product.getItemId());
        bodyValues.put("WarrantyId", "4b93c926-0e32-4681-aba4-ea1a15c89045");
        bodyValues.put("InventoryTypeId", "791a6654-c5f2-11e6-aad6-2c56dc130c0d");
        if(brandId != null){ bodyValues.put("BrandId", brandId); }
        if(!categories.isEmpty()){
            bodyValues.put("ProductCategoryId", categories.getFirst());
            categories.removeFirst();
            for(String category : categories){
                otherCategoriesArray.put(category);
            }
            if(!otherCategoriesArray.isEmpty()){
                bodyValues.put("otherProductCategories", otherCategoriesArray);
            }
        }
        if(!tags.isEmpty()){
            for (String tag : tags) {
                JSONObject tagsObj = new JSONObject();
                tagsObj.put("_id", tag);
                tagsArray.put(tagsObj);
            }
            if (!tagsArray.isEmpty()) {
                bodyValues.put("tags", tagsArray);
            }
        }
        if(syncItem.getDefaultVersionId() != null && (!Objects.equals(product.getWeight(), "") || !Objects.equals(product.getLength(), "") ||
                !Objects.equals(product.getHeight(), "") || !Objects.equals(product.getWidth(), ""))){
            JSONObject productVersionObj = getProductVersionObj(syncItem, product);
            productVersionsArray.put(productVersionObj);
            bodyValues.put("ProductVersions", productVersionsArray);
        }
        return bodyValues;
    }

    private static String getName(InterfaceItems product, String shortDescription) {
        String name = shortDescription;
        boolean hasBrand = name.contains(product.getBrand());
        boolean hasPartNumber = name.contains(product.getPartNumber());
        if(!hasBrand && !Objects.equals(product.getBrand(), "")){
            name = !Objects.equals(name, "") ? name + " "  + product.getBrand() : product.getBrand();
        }
        if(!hasPartNumber && !Objects.equals(product.getPartNumber(), "")){
            name = !Objects.equals(name, "") ? name + " " + product.getPartNumber() : product.getPartNumber();
        }
        return name;
    }

    private static String getShortDescription(String description01, String description02) throws RuntimeException {
        String shortDescription = "";
        if(!Objects.equals(description01, "") && !Objects.equals(description02, "")) {
            shortDescription = description01 + " - " + description02 + " ";
        } else if (!Objects.equals(description01, "")) {
            shortDescription = description01 + " ";
        } else if (!Objects.equals(description02, "")) {
            shortDescription = description02 + " ";
        }
        return shortDescription;
    }

    private static String getCrossReferences(String crossReferences) throws RuntimeException {
        String response = "";
        if(!Objects.equals(crossReferences, "")){
            response = "Equivalente con: " + crossReferences;
        }
        return response;
    }

    private static JSONObject getProductVersionObj(SyncItem syncItem, InterfaceItems product) {
        JSONObject productVersionObj = new JSONObject();
        DecimalFormat weightFmt = new DecimalFormat("0.00");
        productVersionObj.put("_id", syncItem.getDefaultVersionId());
        if(!Objects.equals(product.getWeight(), "")){
            productVersionObj.put("weight", weightFmt.format(Float.parseFloat(product.getWeight()) / 2.20462));
        }
        if(!Objects.equals(product.getLength(), "")){
            productVersionObj.put("length", weightFmt.format(Float.parseFloat(product.getLength())));
        }
        if(!Objects.equals(product.getHeight(), "")){
            productVersionObj.put("height", weightFmt.format(Float.parseFloat(product.getHeight())));
        }
        if(!Objects.equals(product.getWidth(), "")){
            productVersionObj.put("width", weightFmt.format(Float.parseFloat(product.getWidth())));
        }
        productVersionObj.put("InventoryTypeId", "791a6654-c5f2-11e6-aad6-2c56dc130c0d");
        return productVersionObj;
    }

    private void saveSyncProductsInfo(String syncResponse, SyncItem synchronizedProduct, Company company) throws JsonProcessingException {
        try {
            ObjectMapper objMapSyncProducts = new ObjectMapper();
            SyncItem syncItem = objMapSyncProducts.readValue(syncResponse, SyncItem.class);
            syncItem.setCompany(company);
            syncItem.setIntegrationCompany(env.getProperty("integration.company.name"));
            syncItem.setVegSyncStatus("synchronized");
            if(synchronizedProduct != null) {
                syncItem.setRecId(synchronizedProduct.getRecId());
                syncItem.setDefaultVersionId(synchronizedProduct.getDefaultVersionId());
            }
            syncItemRepo.save(syncItem);
        } catch (RuntimeException | JsonProcessingException e) {
            updateMiddlewareSynchronizedProductsWithError(syncResponse, synchronizedProduct, company);
            throw new RuntimeException(e.getMessage());
        }
    }

    private void updateMiddlewareSynchronizedProductsWithError(String syncResponse, SyncItem synchronizedProduct, Company company) throws JsonProcessingException {
        try {
            SyncItem vegMvSynchronizedProduct = new SyncItem();
            if(synchronizedProduct != null){
                vegMvSynchronizedProduct.setRecId(synchronizedProduct.getRecId());
            }
            vegMvSynchronizedProduct.setResponseId(MWUtils.getJsonNodeResponse(syncResponse, "_id"));
            vegMvSynchronizedProduct.setInternalCode(MWUtils.getJsonNodeResponse(syncResponse,"internalCode"));
            vegMvSynchronizedProduct.setCompany(company);
            vegMvSynchronizedProduct.setIntegrationCompany(env.getProperty("integration.company.name"));
            vegMvSynchronizedProduct.setVegSyncStatus("error");
            vegMvSynchronizedProduct.setUpdatedAt(new Date(0));
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
