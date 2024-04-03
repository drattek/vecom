package com.vegusa.veg_mv_integration_midd.msb.service;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.veg_mv_integration_midd.msb.entity.EcommProducts;
import com.vegusa.veg_mv_integration_midd.msb.repository.ProductsRepository;
import com.vegusa.veg_mv_integration_midd.oauth2_0.encrypt_decrypt.AESEncryptDecrypt;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.TokenInfoRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.VegMvIntegrationEndptsRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.utils.MiddUtils;
import org.springframework.transaction.annotation.Transactional;
import jakarta.persistence.EntityManager;
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
import java.util.stream.Stream;


@Service
public class MSBService
{
    private final ProductsRepository productsRepository;
    private final VegMvIntegrationEndptsRepository endptsRepository;
    private final TokenInfoRepository tokenInfoRepository;
    private final EntityManager entityManager;
    private final WebClient webClient;

    private int auxNumberItems = 0;

    @Autowired
    public MSBService(ProductsRepository productsRepository,
                      VegMvIntegrationEndptsRepository endptsRepository,
                      TokenInfoRepository tokenInfoRepository,
                      EntityManager entityManager,
                      WebClient webClient)
    {
        this.productsRepository = productsRepository;
        this.endptsRepository = endptsRepository;
        this.tokenInfoRepository = tokenInfoRepository;
        this.entityManager = entityManager;
        this.webClient = webClient;
    }

    @Transactional(readOnly = true)
    public void processProducts() throws JsonProcessingException, InvalidAlgorithmParameterException, NoSuchPaddingException,
                                    IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException
    {
        String accessToken = MiddUtils.getDecryptedAccessToken(tokenInfoRepository, MiddUtils.getEncryptDecryptInterface());
        ObjectMapper objectMapper = new ObjectMapper();
        JsonNode jsonNodeAppInfo = objectMapper.
                readTree(MiddUtils.getAppInfo(webClient,endptsRepository.getEndPointMuitiVende("GET_APP_INFORMATION"),accessToken));

        if(jsonNodeAppInfo.has("error"))
        {
            System.out.println("An error occurred while obtaining App Information! " + jsonNodeAppInfo.get("error").asText());
        }
        else
        {
            Stream<EcommProducts> productsStream = productsRepository.getProductsToSynchronize();
            String url = endptsRepository.getEndPointMuitiVende("CREATE_PRODUCT")
                                            .replace("{{merchant_id}}",jsonNodeAppInfo.get("MerchantId").asText());

            productsStream.forEach(product -> {

                String response = synchronizeProducts(product,url,accessToken);

                // entityManager.detach(product); //error
            });
        }
    }

    private String synchronizeProducts(EcommProducts product, String url, String accessToken)
    {
        try
        {
            HttpHeaders headers = new HttpHeaders();
            headers.add("Content-Type", "application/json");
            headers.add("Authorization", "Bearer " + accessToken);

            MultiValueMap<String, String> bodyValues = new LinkedMultiValueMap<>();
            bodyValues.add("", "");

            return webClient.post()
                    .uri(url)
                    .headers(h -> h.addAll(headers))
                    .body(BodyInserters.fromFormData(bodyValues))
                    .retrieve()
                    .bodyToMono(String.class)
                    .block();
        }
        catch (WebClientResponseException e)
        {
            return "{ \"error\" : \"" + e.getStatusCode() + " " + e.getMessage()  + "\" }";
        }

    }







}
