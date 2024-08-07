package com.vegusa.veg_mv_integration_midd.msb.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.veg_mv_integration_midd.msb.entity.ECOMProduct;
import com.vegusa.veg_mv_integration_midd.msb.repository.ProductRepository;
import com.vegusa.veg_mv_integration_midd.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegEcomSynchronizedBrands;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegEcomSynchronizedCategories;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegEcomSynchronizedProducts;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VwVegEcommScrapedAdditionalInfo;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.*;
import com.vegusa.veg_mv_integration_midd.veg_middleware.utils.MiddUtils;
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

@Service
public class MSBProductSyncService {
    //MSB-Repository
    private final ProductRepository productsRepository;
    //Middleware - Repository
    private final VegEcomvIntegrationEndptsRepository endptsRepository;
    private final TokenInfoRepository tokenInfoRepository;
    private final VegEcomSynchronizedProductsRepository vegMvSynchronizedProductRepository;
    private final VwVegEcommScrapedAdditionalInfoRepository vwVegEcommScrapedAdditionalInfoRepository;
    private final VegEcomSynchronizedBrandsRepository vegEcomSynchronizedBrandsRepository;
    private final VegEcomSynchronizedCategoriesRepository vegEcomSynchronizedCategoriesRepository;
    //Global
    private final WebClient webClient;
    private final Environment env;
    private final EntityManager entityManager;
    private EncryptDecryptInterface encryptDecryptInterface;
    private String algorithm;

    @Autowired
    public MSBProductSyncService(ProductRepository productsRepository,
                                 VegEcomvIntegrationEndptsRepository endptsRepository,
                                 TokenInfoRepository tokenInfoRepository,
                                 VegEcomSynchronizedProductsRepository vegMvSynchronizedProductRepository,
                                 VwVegEcommScrapedAdditionalInfoRepository vwVegEcommScrapedAdditionalInfoRepository,
                                 VegEcomSynchronizedBrandsRepository vegEcomSynchronizedBrandsRepository,
                                 VegEcomSynchronizedCategoriesRepository vegEcomSynchronizedCategoriesRepository,
                                 WebClient webClient, Environment env,
                                 EntityManager entityManager) {
        this.productsRepository = productsRepository;
        this.endptsRepository = endptsRepository;
        this.tokenInfoRepository = tokenInfoRepository;
        this.vegMvSynchronizedProductRepository = vegMvSynchronizedProductRepository;
        this.vwVegEcommScrapedAdditionalInfoRepository = vwVegEcommScrapedAdditionalInfoRepository;
        this.vegEcomSynchronizedBrandsRepository = vegEcomSynchronizedBrandsRepository;
        this.vegEcomSynchronizedCategoriesRepository = vegEcomSynchronizedCategoriesRepository;
        this.webClient = webClient;
        this.env = env;
        this.entityManager = entityManager;
    }

    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface, String algorithm) {
        this.encryptDecryptInterface = encryptDecryptInterface;
        this.algorithm = algorithm;
    }

    public String processAndUploadProducts() throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException,
            NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException {
        JSONObject response = new JSONObject();
        String accessToken = MiddUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface, env, algorithm);
        JsonNode jsonNodeAppInfo = MiddUtils.validateResponse("An error occurred while obtaining App Information: ",
                        MiddUtils.getAppInfo(webClient, endptsRepository.getIntegrationEndPoint("GET_APP_INFORMATION", "MULTIVENDE"), accessToken));
        ECOMProduct[] products = productsRepository.getDYNProducts();
        String urlCreateProduct = endptsRepository.getIntegrationEndPoint("CREATE_PRODUCT", "MULTIVENDE")
                .replace("{{merchant_id}}", jsonNodeAppInfo.get("MerchantId").asText());
        String urlUpdateProduct = endptsRepository.getIntegrationEndPoint("UPDATE_PRODUCT", "MULTIVENDE");
        for (int it = 0; it < products.length; it++) {
            try {
                HashMap<String, String> syncProduct = new HashMap<>();
                VegEcomSynchronizedProducts synchronizedProduct = vegMvSynchronizedProductRepository.getSynchronizedProductById(products[it].getArticulo());
                SimpleDateFormat formatter = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss");
                String url = (synchronizedProduct == null) ? urlCreateProduct :
                        (formatter.parse(products[it].getModifieddatetime().toString()).after(formatter.parse(synchronizedProduct.getUpdatedAtMv().toString())))
                                ? urlUpdateProduct.replace("{{product_id}}", synchronizedProduct.getIdMvd().toString()) : "not synchronize";
              //  String url = urlUpdateProduct.replace("{{product_id}}", synchronizedProduct.getIdMvd().toString());
                if(url != "not synchronize"){
                    HashMap<String, String> idCatalogs = getCatalogs(accessToken, jsonNodeAppInfo.get("MerchantId").asText(), products[it].getMarca(), products[it].getCateogria());
                    JsonNode jsonNodeSyncProducts = MiddUtils
                            .validateResponse("", synchronizeProducts(accessToken, synchronizedProduct, url, products[it], idCatalogs));
                    updateMiddlewareSynchronizedProducts(jsonNodeSyncProducts, synchronizedProduct);
                    syncProduct.put("ok", "The product " + products[it].getArticulo() + " was synchronized successfully.");
                    response.accumulate("ok", syncProduct);
                    System.out.println("The product " + products[it].getArticulo() + " was synchronized successfully.");
                } else {
                    syncProduct.put("ok", "The product " + products[it].getArticulo() + " doesn´t require to be synchronized.");
                    response.accumulate("synchronized", syncProduct);
                    System.out.println("The product " + products[it].getArticulo() + " doesn´t require to be synchronized.");
                }
            } catch  (RuntimeException | ParseException | JsonProcessingException e) {
                System.err.println("Error when synchronizing the product " + products[it].getArticulo());
                e.printStackTrace();
                HashMap<String, String> syncProductError = new HashMap<>();
                syncProductError.put("error", "Error when synchronizing the product " + products[it].getArticulo());
                syncProductError.put("message", e.getMessage());
                response.accumulate("error", syncProductError);
                if(e.getMessage().contains("401")){
                    accessToken = MiddUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface,  env, algorithm,
                            "Access token was not found in processProducts method.");
                }
            }
        }
        return response.toString();
    }

    private HashMap getCatalogs(String accessToken, String merchantId, String brand, String category){
        try {
            HashMap<String, String> idCatalogs = new HashMap<>();
            VegEcomSynchronizedBrands syncBrandId = vegEcomSynchronizedBrandsRepository.getSynchronizedBrand(brand, "MSB");
            VegEcomSynchronizedCategories syncCategory = vegEcomSynchronizedCategoriesRepository.getSynchronizedCategory(category, "MSB");
            String brandId = syncBrandId != null ? syncBrandId.getIdEcom() : brand != null ?
                    createCatalogValue("BRANDS", accessToken, merchantId, endptsRepository.getIntegrationEndPoint("POST_BRAND", "MULTIVENDE"), brand) : null;
            String categoryId = syncCategory != null ? syncCategory.getIdEcom() : category != null ?
                    createCatalogValue("CATEGORIES", accessToken, merchantId, endptsRepository.getIntegrationEndPoint("CREATE_PRODUCT_CATEGORY", "MULTIVENDE"), category) : null;
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
        MiddUtils.validateResponse("An error occurred while creating the catalog ID value to " + value + " in " + catalog, createdCatalogValue);
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

    private String synchronizeProducts(String accessToken, VegEcomSynchronizedProducts synchronizedProduct, String url, ECOMProduct product, HashMap<String, String> idCatalogs) {
        try {
            HttpHeaders headers = new HttpHeaders();
            headers.add("Content-Type", "application/json");
            headers.add("Authorization", "Bearer " + accessToken);
            MultiValueMap<String, String> bodyValues = new LinkedMultiValueMap<>();
            bodyValues.add("name", product.getDescripcion() + "");
            bodyValues.add("alias", product.getNumParte() + "");
            bodyValues.add("model", product.getNumParte() + "");
            bodyValues.add("description", product.getDescripcion() + "");
            bodyValues.add("code", product.getNumParte() + "");
            bodyValues.add("internalCode", product.getArticulo() + "");
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
            return MiddUtils.getSimpleJSONResponse("error", e.getMessage());
        }
    }

    private void updateMiddlewareSynchronizedProducts(JsonNode jsonNodeResp, VegEcomSynchronizedProducts synchronizedProduct) {
        try {
            ObjectMapper objMapSyncProducts = new ObjectMapper();
            VegEcomSynchronizedProducts vegMvSynchronizedProduct = objMapSyncProducts.readValue(jsonNodeResp.toString(), VegEcomSynchronizedProducts.class);
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

    private void updateMiddlewareSynchronizedProductsWithError(JsonNode jsonNodeResp, VegEcomSynchronizedProducts synchronizedProduct){
        try {
            VegEcomSynchronizedProducts vegMvSynchronizedProduct = new VegEcomSynchronizedProducts();
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
        String accessToken = MiddUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface, env, algorithm);
        String urlUpdateProduct = endptsRepository.getIntegrationEndPoint("UPDATE_PRODUCT", "MULTIVENDE");
        VwVegEcommScrapedAdditionalInfo[] additionalInfo = vwVegEcommScrapedAdditionalInfoRepository.getAdditionalProductsInfo();
        for(int it = 0; it < additionalInfo.length; it++){
            try {
                HashMap<String, String> syncProduct = new HashMap<>();
                VegEcomSynchronizedProducts synchronizedProduct = vegMvSynchronizedProductRepository
                        .getSynchronizedProductById(additionalInfo[it].getInternalProductId());
                String url = urlUpdateProduct.replace("{{product_id}}", additionalInfo[it].getIdMv());
                JsonNode jsonNodeSyncProducts = MiddUtils
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
                    accessToken = MiddUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface, env, algorithm,
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
            return MiddUtils.getSimpleJSONResponse("error", e.getMessage());
        }
    }


}
