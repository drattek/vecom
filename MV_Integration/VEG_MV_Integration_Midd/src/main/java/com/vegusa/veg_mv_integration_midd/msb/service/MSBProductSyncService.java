package com.vegusa.veg_mv_integration_midd.msb.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.veg_mv_integration_midd.msb.repository.ProductsRepository;
import com.vegusa.veg_mv_integration_midd.oauth2_0.encrypt_decrypt.EncryptDecryptInterface;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegMvSynchronizedProduct;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.TokenInfoRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.VegMvIntegrationEndptsRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.VegMvSynchronizedProductRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.utils.MiddUtils;
import org.springframework.core.env.Environment;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.util.LinkedMultiValueMap;
import org.springframework.util.MultiValueMap;
import org.springframework.web.reactive.function.BodyInserters;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.reactive.function.client.WebClientResponseException;
import org.springframework.http.HttpHeaders;
import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.text.ParseException;
import java.text.SimpleDateFormat;
import java.util.Iterator;
import java.util.stream.Stream;


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

    @Autowired
    public MSBProductSyncService(ProductsRepository productsRepository,
                                 VegMvIntegrationEndptsRepository endptsRepository,
                                 TokenInfoRepository tokenInfoRepository,
                                 VegMvSynchronizedProductRepository vegMvSynchronizedProductRepository,
                                 WebClient webClient, Environment env) {
        this.productsRepository = productsRepository;
        this.endptsRepository = endptsRepository;
        this.tokenInfoRepository = tokenInfoRepository;
        this.vegMvSynchronizedProductRepository = vegMvSynchronizedProductRepository;
        this.webClient = webClient;
        this.env = env;
    }

    public void setEncryptDecryptInterface(EncryptDecryptInterface encryptDecryptInterface) {
        this.encryptDecryptInterface = encryptDecryptInterface;
    }

    @Transactional(readOnly = false)
    public void processProducts()
            throws JsonProcessingException, InvalidAlgorithmParameterException, NoSuchPaddingException, IllegalBlockSizeException,
            NoSuchAlgorithmException, BadPaddingException, InvalidKeyException, ParseException {
        String accessToken = MiddUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface);
        ObjectMapper objMapAppInfo = new ObjectMapper();
        JsonNode jsonNodeAppInfo = objMapAppInfo.
                readTree(MiddUtils.getAppInfo(webClient, endptsRepository.getEndPointMuitiVende("GET_APP_INFORMATION"), accessToken));

        if(jsonNodeAppInfo.has("error")) {
            System.out.println("An error occurred while obtaining App Information. " + jsonNodeAppInfo.get("error").asText());
        } else {
            Stream<Object[]> productsStream = productsRepository.getProductsToSynchronize();
            String urlCreateProduct = endptsRepository.getEndPointMuitiVende("CREATE_PRODUCT")
                    .replace("{{merchant_id}}", jsonNodeAppInfo.get("MerchantId").asText());
            String urlUpdateProduct = endptsRepository.getEndPointMuitiVende("UPDATE_PRODUCT");

            final int[] auxCont = {1}; //PRUEBAS

            for (Iterator<Object[]> it = productsStream.iterator(); it.hasNext(); ) {
                Object[] product = it.next();

                if (auxCont[0] >= 11 && auxCont[0] <= 15) { //PRUEBAS

                    VegMvSynchronizedProduct synchronizedProduct = vegMvSynchronizedProductRepository.getSynchronizedProduct(product[4].toString());
                    ObjectMapper objMapSyncProducts = new ObjectMapper();
                    String response = synchronizeProducts(accessToken, product, urlCreateProduct, urlUpdateProduct, synchronizedProduct);
                    JsonNode jsonNodeSyncProducts = objMapSyncProducts.readTree(response);

                    if (jsonNodeSyncProducts.has("error") &&
                            jsonNodeSyncProducts.get("error").toString().contains("401 UNAUTHORIZED")) {
                        accessToken = MiddUtils.getDecryptedAccessToken(tokenInfoRepository, encryptDecryptInterface);
                        response = synchronizeProducts(accessToken, product, urlCreateProduct, urlUpdateProduct, synchronizedProduct);
                        jsonNodeSyncProducts = objMapSyncProducts.readTree(response);
                        updateMiddlewareSynchronizedProducts(jsonNodeSyncProducts, response, synchronizedProduct);
                    }
                    else {
                        updateMiddlewareSynchronizedProducts(jsonNodeSyncProducts, response, synchronizedProduct);
                    }

                } //PRUEBAS
                System.out.println("Contador: " + auxCont[0]); //PRUEBAS
                auxCont[0] = auxCont[0] + 1; //PRUEBAS

                // entityManager.detach(product); //error
            }
        }
    }

    private String synchronizeProducts(String accessToken, Object[] product, String urlCreate, String urlUpdate,
                                       VegMvSynchronizedProduct synchronizedProduct)
            throws ParseException {
        SimpleDateFormat formatter = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss");
        String url = (synchronizedProduct == null) ? urlCreate :
                    //    (5 > 3) // PRUEBAS
                        (formatter.parse(product[5].toString()).after(formatter.parse(synchronizedProduct.getUpdatedAtMv().toString())))
                                ? urlUpdate.replace("{{product_id}}", synchronizedProduct.getIdMvd().toString()) : "not synchronize";

        if(url != "not synchronize") {
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

                if(synchronizedProduct == null) {
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

            } catch (WebClientResponseException e) {
                return "{ \"error\" : \"" + e.getStatusCode() + " " + e.getMessage() + "\" }";
            }

        } else {
            return "{ \"error\" : \"" + "The product " + synchronizedProduct.getInternalCode().toString() + " was already synchronized." + "\" }";
        }
    }

    private void updateMiddlewareSynchronizedProducts(JsonNode jsonNodeSyncProducts, String response, VegMvSynchronizedProduct synchronizedProduct)
            throws JsonProcessingException {
        if (jsonNodeSyncProducts.has("error")) {
            System.out.println("An error occurred while synchronizing the products! " + jsonNodeSyncProducts.get("error").asText());
        } else {
            ObjectMapper objMapSyncProducts = new ObjectMapper();
            VegMvSynchronizedProduct vegMvSynchronizedProduct = (synchronizedProduct == null) ?
                    objMapSyncProducts.readValue(response, VegMvSynchronizedProduct.class) : synchronizedProduct;
            vegMvSynchronizedProduct.setVegBusinessUnit("MSB");
            vegMvSynchronizedProduct.setIntegrationCompany(env.getProperty("integration.company.name"));
            vegMvSynchronizedProductRepository.save(vegMvSynchronizedProduct);
            System.out.println("Synchronized product: " + vegMvSynchronizedProduct.getInternalCode());
        }
    }
}
