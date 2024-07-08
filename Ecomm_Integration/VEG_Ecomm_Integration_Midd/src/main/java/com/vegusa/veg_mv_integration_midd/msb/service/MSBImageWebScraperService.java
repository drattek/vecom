package com.vegusa.veg_mv_integration_midd.msb.service;

import com.vegusa.veg_mv_integration_midd.msb.entity.ECOMProduct;
import com.vegusa.veg_mv_integration_midd.msb.repository.ProductRepository;
import com.vegusa.veg_mv_integration_midd.msb.repository.ProductsRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VwVegImagesByProduct;
import jakarta.persistence.EntityManager;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpHeaders;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.reactive.function.client.WebClientResponseException;

import java.util.Iterator;
import java.util.function.Supplier;
import java.util.stream.Stream;

@Service
public class MSBImageWebScraperService {

    private final ProductsRepository productsRepository;
    private final ProductRepository productRepository;
    private final EntityManager entityManager;

    private final WebClient webClient;

    @Autowired
    public MSBImageWebScraperService(ProductsRepository productsRepository,
                                     ProductRepository productRepository,
                                     EntityManager entityManager,
                                     WebClient webClient){
        this.productsRepository = productsRepository;
        this.productRepository = productRepository;
        this.entityManager = entityManager;
        this.webClient = webClient;
    }

    @Transactional(readOnly = false)
    public String getJsonRequest(String userEmail, String userPass) throws RuntimeException {
        JSONObject request = new JSONObject();
        request.put("userEmail", userEmail);
        request.put("userPass", userPass);
        JSONArray productsArray = new JSONArray();
        Stream<Object[]> productsStream = productsRepository.getProductsToSynchronize();
        for (Iterator<Object[]> it = productsStream.iterator(); it.hasNext(); ) {
            Object[] productStream = it.next();
            JSONObject product = new JSONObject();
            product.put("internalProductId", productStream[4].toString());
            product.put("productName", productStream[0].toString());
            product.put("productId", productStream[2].toString());
            productsArray.put(product);
        }
        request.put("products",productsArray);
        return request.toString();
    }


    /*
    Así debe quedar con la nueva vista
    @Transactional(readOnly = false)
    public String getJsonRequest(String userEmail, String userPass) throws RuntimeException {
        JSONObject request = new JSONObject();
        request.put("userEmail", userEmail);
        request.put("userPass", userPass);
        JSONArray productsArray = new JSONArray();
        Stream<ECOMProduct> productsStream = productRepository.getStreamProductsToSynchronize();
        productsStream.forEach(product -> {
            JSONObject productResponse = new JSONObject();
            productResponse.put("internalProductId", product.getArticulo());
            productResponse.put("productName", product.getDescripción());
            productResponse.put("productId", product.getNumParte());
            productsArray.put(productResponse);
            entityManager.detach(product);
        });
        request.put("products", productsArray);
        return request.toString();
    }
    */

    public String getTVHScrapedImages(String bodyValues) throws WebClientResponseException {
        HttpHeaders headers = new HttpHeaders();
        headers.add("Content-Type", "application/json");
        return webClient.post()
                .uri("http://localhost:8080/veg-web-scraper/image")
                .headers(h -> h.addAll(headers))
                .bodyValue(bodyValues)
                .retrieve()
                .bodyToMono(String.class)
                .block();
    }

    public String getTVHProductsInfo(String bodyValues) throws WebClientResponseException {
        HttpHeaders headers = new HttpHeaders();
        headers.add("Content-Type", "application/json");
        return webClient.post()
                .uri("http://localhost:8080/veg-web-scraper/tvh-additional-info")
                .headers(h -> h.addAll(headers))
                .bodyValue(bodyValues)
                .retrieve()
                .bodyToMono(String.class)
                .block();
    }

    public String getUCAProductsInfo(String bodyValues) throws WebClientResponseException {
        HttpHeaders headers = new HttpHeaders();
        headers.add("Content-Type", "application/json");
        return webClient.post()
                .uri("http://localhost:8080/veg-web-scraper/uca-additional-info")
                .headers(h -> h.addAll(headers))
                .bodyValue(bodyValues)
                .retrieve()
                .bodyToMono(String.class)
                .block();
    }

}
