package com.vegusa.middleware.service;

import com.vegusa.middleware.repository.local.ItemScrapedInfoRepository;
import com.vegusa.middleware.repository.local.ScrapedImageRepository;
import com.vegusa.middleware.model.erp.DYNProduct;
import com.vegusa.middleware.repository.erp.DYNProductRepository;
import jakarta.persistence.EntityManager;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpHeaders;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.reactive.function.client.WebClientResponseException;

@Service
public class WebScraperService {

    private final DYNProductRepository dynProducts;
    private final ScrapedImageRepository scrapedImageRepo;
    private final ItemScrapedInfoRepository itemScrapedInfoRepo;
    private final WebClient webClient;

    @Autowired
    public WebScraperService(DYNProductRepository dynProducts,
                             ScrapedImageRepository scrapedImageRepo,
                             ItemScrapedInfoRepository itemScrapedInfoRepo,
                             EntityManager entityManager,
                             WebClient webClient){
        this.dynProducts = dynProducts;
        this.scrapedImageRepo = scrapedImageRepo;
        this.itemScrapedInfoRepo = itemScrapedInfoRepo;
        this.webClient = webClient;
    }

    @Transactional(readOnly = false)
    public String getJsonRequest(String userEmail, String userPass) throws RuntimeException {
        JSONObject request = new JSONObject();
        request.put("userEmail", userEmail);
        request.put("userPass", userPass);
        JSONArray productsArray = new JSONArray();
        String art, desc, numParte;
        DYNProduct[] dataSourceInfo = dynProducts.getDYNProducts();
        for (DYNProduct ecomProduct : dataSourceInfo) {
            art = ecomProduct.getArticulo() != null ? ecomProduct.getArticulo() : "";
            desc = ecomProduct.getDescripcion() != null ? ecomProduct.getDescripcion() : "";
            numParte = ecomProduct.getNumParte() != null ? ecomProduct.getNumParte() : "";
            JSONObject product = new JSONObject();
            product.put("itemId", art);
            product.put("productName", desc);
            product.put("partNumber", numParte);
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
