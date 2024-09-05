package com.vegusa.veg_ecomm_webscraperimages.controller;

import com.vegusa.veg_ecomm_webscraperimages.service.WebScraperService;
import org.json.JSONArray;
import org.json.JSONObject;
import org.openqa.selenium.WebDriver;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;

import java.util.HashMap;
import java.util.List;

@RestController
@RequestMapping(value = "veg-web-scraper")
public class WebScraperController {
    private final WebScraperService webScraperService;

    @Autowired
    public WebScraperController(WebScraperService webScraperService){
        this.webScraperService = webScraperService;
    }

    @PostMapping(value="/tvh-images")
    public String scrapeTVHImages(@RequestBody String productsReq){
        JSONObject response = new JSONObject();
        JSONObject productsInfo = new JSONObject(productsReq);
        JSONArray productsList = productsInfo.getJSONArray("products");
        WebDriver loginDriver = webScraperService.loginPageTVH(productsInfo.get("userEmail").toString(), productsInfo.get("userPass").toString());
        for (int i = 0; i < productsList.length(); i++) {
            try {
                HashMap<String, String> imageUrlToUpload;
                HashMap<String, String> imageUrlUploaded;
                imageUrlToUpload = webScraperService.scraperTVHPage(productsList.getJSONObject(i).getString("internalProductId"),
                        productsList.getJSONObject(i).getString("productId"), loginDriver, "images");
                if (imageUrlToUpload.containsKey("error")) {
                    System.err.println(imageUrlToUpload.get("error") + " " + imageUrlToUpload.get("message"));
                    response.accumulate("unsavedProductImages", imageUrlToUpload);
                } else {
                    imageUrlUploaded = webScraperService.uploadImageToAzure(productsList.getJSONObject(i).getString("internalProductId"),
                            productsList.getJSONObject(i).getString("productId"), imageUrlToUpload.get("image"));
                    saveInfo(response, productsList, i, imageUrlUploaded, "");
                }
            } catch (RuntimeException e) {
                System.err.println("An error occurred in the sleep method to the product: " + productsList.getJSONObject(i).getString("internalProductId"));
            }
        }
        return response.toString();
    }

    @PostMapping(value="/scrape-uca-images")
    public String scrapeUCAImages(@RequestBody String request){
        try {
            JSONObject response = new JSONObject(), requestInfo = new JSONObject(request);
            JSONArray productsList = requestInfo.getJSONArray("products");
            WebDriver loginDriver = webScraperService.getUCADriverLoginPage(requestInfo.get("userEmail").toString(), requestInfo.get("userPass").toString());
            for(int i = 0; i < productsList.length(); i++) {
                System.out.println("ENTER TO FOR CYCLE...");
                try {
                    List<String> imageUrlToUpload = webScraperService.scraperUCAPage(productsList.getJSONObject(i).getString("internalProductId"),
                            productsList.getJSONObject(i).getString("productId"), loginDriver);
                    String partNumberFound = imageUrlToUpload.getFirst();
                    HashMap<String, String> imageUrlUploaded;
                    imageUrlToUpload.removeFirst();
                    for (String image : imageUrlToUpload) {
                        imageUrlUploaded = webScraperService.uploadImageToAzure(productsList.getJSONObject(i).getString("internalProductId"),
                                productsList.getJSONObject(i).getString("productId"), image);
                        saveInfo(response, productsList, i, imageUrlUploaded, partNumberFound);
                    }
                } catch (RuntimeException e) {
                    System.err.println("An error occurred while scraping / saving the images for item: " + productsList.getJSONObject(i).getString("internalProductId") + " " + e.getMessage());
                    response.accumulate("unsavedProductImages", e.getMessage());
                }
                System.out.println("END SCRAPE CYCLE TO ..." + productsList.getJSONObject(i).getString("internalProductId"));
            }
            return response.toString();
        } catch (RuntimeException e){
            System.err.println("An error occurred while scraping uca images.");
            throw new ResponseStatusException(HttpStatus.INTERNAL_SERVER_ERROR, e.getMessage(), e);
        }
    }

    private void saveInfo(JSONObject response, JSONArray productsList, int i, HashMap<String, String> imageUrlUploaded, String partNumberFound) throws RuntimeException{
        HashMap<String, String> imageSaved;
        if(imageUrlUploaded.containsKey("error")) {
            System.err.println(imageUrlUploaded.get("error") + " " + imageUrlUploaded.get("message"));
            response.accumulate("unsavedProductImages", imageUrlUploaded);
        } else {
            imageSaved = webScraperService.saveUploadedImageInfo(productsList.getJSONObject(i).getString("internalProductId"),
                    productsList.getJSONObject(i).getString("productName"), productsList.getJSONObject(i).getString("productId"),
                    imageUrlUploaded.get("blobName"), imageUrlUploaded.get("urlSavedImage"), partNumberFound);
            if (imageSaved.containsKey("error")) {
                System.err.println(imageSaved.get("error") + " " + imageSaved.get("message"));
                response.accumulate("unsavedProductImages", imageSaved);
            } else{
                response.accumulate("savedProductImages", imageSaved);
            }
        }
    }

    @PostMapping(value="/tvh-additional-info")
    public String scrapeTVHAdditionalInfo(@RequestBody String productsReq) {
        JSONObject response = new JSONObject();
        JSONObject productsInfo = new JSONObject(productsReq);
        JSONArray productsList = productsInfo.getJSONArray("products");
        WebDriver loginDriver = webScraperService.loginPageTVH(productsInfo.get("userEmail").toString(), productsInfo.get("userPass").toString());
        for (int it = 0; it < productsList.length(); it++) {
            try {
                HashMap<String, String> scrapedInfo;
                HashMap<String, String> savedInfo;
                scrapedInfo = webScraperService.scraperTVHPage(productsList.getJSONObject(it).getString("internalProductId"),
                        productsList.getJSONObject(it).getString("productId"), loginDriver, "productInfo");
                if (scrapedInfo.containsKey("error")) {
                    System.err.println(scrapedInfo.get("error") + " " + scrapedInfo.get("message"));
                    response.accumulate("unsavedProductInfo", scrapedInfo);
                } else{
                    savedInfo = webScraperService.saveScrapedProductInfo(productsList.getJSONObject(it).getString("internalProductId"),
                            productsList.getJSONObject(it).getString("productName"), productsList.getJSONObject(it).getString("productId"),
                            scrapedInfo);
                    System.out.println("Products Info saved.");
                    if (savedInfo.containsKey("error")) {
                        System.err.println(savedInfo.get("error") + " " + savedInfo.get("message"));
                        response.accumulate("unsavedProductInfo", savedInfo);
                    } else{
                        response.accumulate("savedProductInfo", savedInfo);
                    }
                }
                if((it + 1) % 100 == 0){
                    System.out.println("Is in Sleep time!");
                    Thread.sleep(150000); //5 minutes
                }
            } catch (InterruptedException e) {
                System.err.println("An error occurred in the sleep method to the product: " + productsList.getJSONObject(it).getString("internalProductId"));
            }
        }
        return response.toString();
    }

    @PostMapping(value="/uca-additional-info")
    public String getAndSaveUCAInfo(@RequestBody String productsReq) {
        JSONObject response = new JSONObject();
        try {
            JSONObject productsInfo = new JSONObject(productsReq);
            JSONArray productsList = productsInfo.getJSONArray("products");
            for (int it = 0; it < productsList.length(); it++) {
                try {
                    HashMap<String, String> obtainedInfo = new HashMap<>();
                    String productInfo = webScraperService.getLogisNextInfo(productsList.getJSONObject(it).getString("productId"));
                    webScraperService.saveLogisNextInfo(productsList.getJSONObject(it).getString("internalProductId"),
                            productsList.getJSONObject(it).getString("productName"), productsList.getJSONObject(it).getString("productId"), productInfo);
                    obtainedInfo.put("message", "Info successfully saved to " + productsList.getJSONObject(it).getString("internalProductId"));
                    response.accumulate("obtainedInfo", obtainedInfo);
                    System.out.println("Info successfully saved to " + productsList.getJSONObject(it).getString("internalProductId") + "/" + productsList.getJSONObject(it).getString("productId"));
                } catch (RuntimeException e) {
                    HashMap<String, String> errorInfo = new HashMap<>();
                    errorInfo.put("error", "Error occurred while obtaining info to " + productsList.getJSONObject(it).getString("internalProductId"));
                    errorInfo.put("message", e.getMessage());
                    response.accumulate("error", errorInfo);
                    System.err.println("Error occurred while obtaining info to " + productsList.getJSONObject(it).getString("internalProductId") + "/" + productsList.getJSONObject(it).getString("productId"));
                    System.err.println(e.getMessage());
                }
            }
        } catch(RuntimeException e){
            return "An error occurred while obtaining UCA info: " + "\r" + e.getMessage();
        }
        return response.toString();
    }

    @PostMapping(value="/fix-cross-references")
    public void fixCrossReferences(){
        webScraperService.fixCrossReferences();
    }

    @PostMapping(value="/add-founded-part-number")
    public void addFoundedPartNumberColumn(){
        webScraperService.addFoundedPartNumberColumn();
        System.out.println("The work ends!");
    }




}
