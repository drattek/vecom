package com.vegusa.veg_mv_integration_midd.msb.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.veg_mv_integration_midd.msb.repository.ProductsRepository;
import com.vegusa.veg_mv_integration_midd.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.TokenInfo;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegMvSynchronizedProduct;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.TokenInfoRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.VegMvIntegrationEndptsRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.VegMvSynchronizedProductRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.utils.MiddUtils;
import jakarta.persistence.EntityManager;
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
import javax.crypto.SecretKey;
import javax.crypto.spec.IvParameterSpec;
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
    private final ProductsRepository productsRepository;
    //Middleware - Repository
    private final VegMvIntegrationEndptsRepository endptsRepository;
    private final TokenInfoRepository tokenInfoRepository;
    private final VegMvSynchronizedProductRepository vegMvSynchronizedProductRepository;
    private final WebClient webClient;
    private final Environment env;
    private EncryptDecryptInterface encryptDecryptInterface;
    private final EntityManager entityManager;

    @Autowired
    public MSBProductSyncService(ProductsRepository productsRepository,
                                 VegMvIntegrationEndptsRepository endptsRepository,
                                 TokenInfoRepository tokenInfoRepository,
                                 VegMvSynchronizedProductRepository vegMvSynchronizedProductRepository,
                                 WebClient webClient, Environment env,
                                 EntityManager entityManager) {
        this.productsRepository = productsRepository;
        this.endptsRepository = endptsRepository;
        this.tokenInfoRepository = tokenInfoRepository;
        this.vegMvSynchronizedProductRepository = vegMvSynchronizedProductRepository;
        this.webClient = webClient;
        this.env = env;
        this.entityManager = entityManager;
    }

    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface) {
        this.encryptDecryptInterface = encryptDecryptInterface;
    }

    public String processProducts() throws RuntimeException, InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException,
            NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, JsonProcessingException {
        JSONObject response = new JSONObject();
        String accessToken = MiddUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface);
        JsonNode jsonNodeAppInfo = MiddUtils
                .validateResponse("An error occurred while obtaining App Information: ",
                        MiddUtils.getAppInfo(webClient, endptsRepository.getEndPointMuitiVende("GET_APP_INFORMATION"), accessToken));
        Object[][] productsStream = productsRepository.getProductsToSynchronizeTEST();
        String urlCreateProduct = endptsRepository.getEndPointMuitiVende("CREATE_PRODUCT")
                .replace("{{merchant_id}}", jsonNodeAppInfo.get("MerchantId").asText());
        String urlUpdateProduct = endptsRepository.getEndPointMuitiVende("UPDATE_PRODUCT");
        for (int it = 0; it < productsStream.length; it++) {
            try {
                HashMap<String, String> syncProduct = new HashMap<>();
                Object[] product = productsStream[it];
                VegMvSynchronizedProduct synchronizedProduct = vegMvSynchronizedProductRepository.getSynchronizedProductById(product[4].toString());
                SimpleDateFormat formatter = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss");
                String url = (synchronizedProduct == null) ? urlCreateProduct :
                        (formatter.parse(product[5].toString()).after(formatter.parse(synchronizedProduct.getUpdatedAtMv().toString())))
                                ? urlUpdateProduct.replace("{{product_id}}", synchronizedProduct.getIdMvd().toString()) : "not synchronize";
                if(url != "not synchronize"){
                    JsonNode jsonNodeSyncProducts = MiddUtils
                            .validateResponse("", synchronizeProducts(accessToken, product, url, synchronizedProduct));
                    updateMiddlewareSynchronizedProducts(jsonNodeSyncProducts, synchronizedProduct);
                    syncProduct.put("ok", "The product " + product[4].toString() + " was synchronized successfully.");
                    response.accumulate("ok", syncProduct);
                    System.out.println("The product " + product[4].toString() + " was synchronized successfully.");
                } else {
                    syncProduct.put("ok", "The product " + product[4].toString() + " doesn´t require to be synchronized.");
                    response.accumulate("synchronized", syncProduct);
                    System.out.println("The product " + product[4].toString() + " doesn´t require to be synchronized.");
                }
            } catch  (RuntimeException | ParseException | JsonProcessingException e) {
                System.err.println("Error when synchronizing the product " + productsStream[it][4]);
                e.printStackTrace();
                HashMap<String, String> syncProductError = new HashMap<>();
                syncProductError.put("error", "Error when synchronizing the product " + productsStream[it][4]);
                syncProductError.put("message", e.getMessage());
                response.accumulate("error", syncProductError);
                if(e.getMessage().contains("401")){
                    accessToken = getDecryptedAccessToken();
                }
            }
        }
        return response.toString();
    }

    private String getDecryptedAccessToken() {
        try {
            TokenInfo tokenInfo =  tokenInfoRepository.findById(1).orElse(null);
            if(tokenInfo != null) {
                SecretKey key = MiddUtils.convertStringToSecretKey(tokenInfo.getSecretKey());
                IvParameterSpec ivParameterSpec = MiddUtils.convertStringToIvParameterSpec(tokenInfo.getInitializationVector());
                String algorithm = "AES/CBC/PKCS5Padding";
                return encryptDecryptInterface.decrypt(algorithm, tokenInfo.getCipherAccessToken(), key, ivParameterSpec);
            }
        } catch (RuntimeException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                 NoSuchAlgorithmException | BadPaddingException | InvalidKeyException e){
            System.err.println("Access token was not found in processProducts method.");
        }
        return "";
    }

    private String synchronizeProducts(String accessToken, Object[] product, String url, VegMvSynchronizedProduct synchronizedProduct) {
        try {
            HttpHeaders headers = new HttpHeaders();
            headers.add("Content-Type", "application/json");
            headers.add("Authorization", "Bearer " + accessToken);
            MultiValueMap<String, String> bodyValues = new LinkedMultiValueMap<>();
            bodyValues.add("name", product[0] + "");
            bodyValues.add("alias", product[1] + "");
            bodyValues.add("model", product[2] + "");
            bodyValues.add("description", product[3] + "");
            bodyValues.add("code", product[4] + "");
            bodyValues.add("internalCode", product[4] + "");
            bodyValues.add("InventoryTypeId", "791a6654-c5f2-11e6-aad6-2c56dc130c0d");
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

    private void updateMiddlewareSynchronizedProducts(JsonNode jsonNodeResp, VegMvSynchronizedProduct synchronizedProduct) {
        try {
            ObjectMapper objMapSyncProducts = new ObjectMapper();
            VegMvSynchronizedProduct vegMvSynchronizedProduct = objMapSyncProducts.readValue(jsonNodeResp.toString(), VegMvSynchronizedProduct.class);
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

    private void updateMiddlewareSynchronizedProductsWithError(JsonNode jsonNodeResp, VegMvSynchronizedProduct synchronizedProduct){
        try {
            VegMvSynchronizedProduct vegMvSynchronizedProduct = new VegMvSynchronizedProduct();
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

}
