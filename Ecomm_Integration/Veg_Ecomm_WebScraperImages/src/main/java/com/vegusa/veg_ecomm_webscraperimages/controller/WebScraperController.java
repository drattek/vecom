package com.vegusa.veg_ecomm_webscraperimages.controller;

import com.vegusa.veg_ecomm_webscraperimages.service.WebScraperService;
import org.json.JSONArray;
import org.json.JSONObject;
import org.openqa.selenium.WebDriver;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.*;
import java.util.HashMap;

@RestController
@RequestMapping(value = "veg-web-scraper")
public class WebScraperController {
    private final WebScraperService webScraperService;

    @Autowired
    public WebScraperController(WebScraperService webScraperService){
        this.webScraperService = webScraperService;
    }

    @PostMapping(value="/image")
    public String scrapeImage(@RequestBody String productsReq) throws InterruptedException {
        JSONObject response = new JSONObject();
        JSONObject productsInfo = new JSONObject(productsReq);
        JSONArray productsList = productsInfo.getJSONArray("products");
        WebDriver loginDriver = webScraperService.loginPageTVH(productsInfo.get("userEmail").toString(), productsInfo.get("userPass").toString());

        for (int i = 0; i < productsList.length(); i++) {
            HashMap<String, String> imageUrlToUpload;
            HashMap<String, String> imageUrlUploaded;
            HashMap<String, String> imageSaved;

            imageUrlToUpload = webScraperService.scraperTVH(productsList.getJSONObject(i).getString("internalProductId"),
                    productsList.getJSONObject(i).getString("productId"), loginDriver);
            if (imageUrlToUpload.containsKey("error")) {
                System.err.println(imageUrlToUpload.get("error") + " " + imageUrlToUpload.get("message"));
                response.accumulate("unsavedProducts", imageUrlToUpload);
            } else {
                imageUrlUploaded = webScraperService.uploadImageToAzure(productsList.getJSONObject(i).getString("internalProductId"),
                        productsList.getJSONObject(i).getString("productId"), imageUrlToUpload.get("image"));
                if (imageUrlUploaded.containsKey("error")) {
                    System.err.println(imageUrlUploaded.get("error") + " " + imageUrlUploaded.get("message"));
                    response.accumulate("unsavedProducts", imageUrlUploaded);
                } else {
                    imageSaved = webScraperService.saveUploadedImageInfo(productsList.getJSONObject(i).getString("internalProductId"),
                            productsList.getJSONObject(i).getString("productName"), productsList.getJSONObject(i).getString("productId"),
                            imageUrlUploaded.get("blobName"), imageUrlUploaded.get("urlSavedImage"));
                    if (imageSaved.containsKey("error")) {
                        System.err.println(imageSaved.get("error") + " " + imageSaved.get("message"));
                        response.accumulate("unsavedProducts", imageSaved);
                    } else{
                        response.accumulate("savedProducts", imageSaved);
                    }
                }
            }
     //       if((i + 1) % 30 == 0){
       //         Thread.sleep(1440000); //24 minutes
        //    }
        }
        return response.toString();
    }

}
