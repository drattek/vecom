package com.vegusa.veg_ecomm_webscraperimages.service;

import com.azure.storage.blob.BlobContainerClient;
import com.azure.storage.blob.BlobContainerClientBuilder;
import com.azure.storage.blob.specialized.BlockBlobClient;
import com.vegusa.veg_ecomm_webscraperimages.entity.EcommImageProducts;
import com.vegusa.veg_ecomm_webscraperimages.repository.EcommImageProductsRepository;
import jakarta.persistence.PersistenceException;
import org.json.JSONObject;
import org.jsoup.Jsoup;
import org.jsoup.nodes.Document;
import org.jsoup.nodes.Element;
import org.openqa.selenium.By;
import org.openqa.selenium.WebDriver;
import org.openqa.selenium.WebElement;
import org.openqa.selenium.chrome.ChromeDriver;
import org.openqa.selenium.chrome.ChromeOptions;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.core.env.Environment;
import org.springframework.stereotype.Service;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.net.URL;
import java.util.Dictionary;
import java.util.HashMap;

@Service
public class WebScraperService {
    private final Environment env;
    private final EcommImageProductsRepository ecommImageProductsRepository;

    @Autowired
    public WebScraperService(EcommImageProductsRepository ecommImageProductsRepository, Environment env){
        this.ecommImageProductsRepository = ecommImageProductsRepository;
        this.env = env;
    }

    public WebDriver loginPageTVH(String userEmail, String userPass){
        ChromeOptions chromeOptions = new ChromeOptions();
        chromeOptions.addArguments("--headless");
        chromeOptions.addArguments("window-size=1200,1100");
        //Create web driver
        WebDriver driver = new ChromeDriver(chromeOptions);
        //System.setProperty("webdriver.chrome.driver", "Path of the chrome driver");

        //LOGIN PAGE
        driver.get(env.getProperty("integration.env.tvh-login-page"));
        //Find and set username
        WebElement username = driver.findElement(By.id("tbUserName"));
        username.sendKeys(userEmail);
        //Find and set password
        WebElement password = driver.findElement(By.id("tbPassword"));
        password.sendKeys(userPass);
        //Find and click login button
        WebElement loginButton = driver.findElement(By.id("lbtnLogin"));
        loginButton.click();
        return driver;
    }

    public HashMap scraperTVH(String internalProductId, String productId, WebDriver driver) {
        HashMap<String, String> response = new HashMap<>();
        try {
            //HOME PAGE
            driver.get(env.getProperty("integration.env.tvh-home-page"));
            //Close popup window
            WebElement popupCloseButton = driver.findElement(By.className("_close_button"));
            popupCloseButton.click();
            //Search text box
            WebElement searchText = driver.findElement(By.id("ctl00_tbHeaderSearchParm"));
            searchText.sendKeys(productId);
            //Search button
            WebElement searchButton = driver.findElement(By.id("lbtnHeaderSearch"));
            searchButton.click();
            //Click image result
            WebElement imageBlock = driver.findElement(By.id("ctl00_cphContent_rptrItemSearch_ctl00_lbtnSearchItem"));
            imageBlock.click();

            //SCRAPE PAGE
            //Retrieve the page source from Selenium
            String pageSource = driver.getPageSource();
            //Parse the page source with jsoup
            Document page = Jsoup.parse(pageSource);
            Element image = page.getElementById("IIProductImage");
            response.put("image", image.attr("data-image-list-x").substring(2, image.attr("data-image-list-x").length() - 2));
            //driver.close();
            //driver.quit();
            System.out.println("The scraper to product " + productId + " ended.");
            System.out.println("Found image: " + image.attr("data-image-list-x").substring(2, image.attr("data-image-list-x").length() - 2));
        } catch(Exception e) {
            response.put("error", "An error occurred while scraping the image: " + internalProductId + "/" + productId);
            response.put("message", e.toString());
        }
        return response;
    }

    public HashMap uploadImageToAzure(String internalProductId, String productId, String url) {
        HashMap<String, String> response = new HashMap<>();
        try {
            URL urlImage = new URL(url);
            String connString = "DefaultEndpointsProtocol=https;" +
                                        "AccountName=" + env.getProperty("integration.env.azure.storage.account-name") + ";" +
                                        "AccountKey=" + env.getProperty("integration.env.azure.storage.account-key") + ";" +
                                        "EndpointSuffix=core.windows.net";
            BlobContainerClient container = new BlobContainerClientBuilder()
                                                .connectionString(connString)
                                                .containerName(env.getProperty("integration.env.azure.storage.container-name"))
                                                .buildClient();

            String imageBlobName = getBlobName(internalProductId, productId);

            BlockBlobClient blockBlobClient = container.getBlobClient(imageBlobName + ".png")
                                                            .getBlockBlobClient();
            ByteArrayOutputStream outputStream = new ByteArrayOutputStream();
            urlImage.openStream().transferTo(outputStream);

            ByteArrayInputStream dataStream = new ByteArrayInputStream(outputStream.toByteArray());
            blockBlobClient.upload(dataStream, outputStream.size(),true);
            response.put("blobName", blockBlobClient.getBlobName());
            response.put("urlSavedImage", blockBlobClient.getBlobUrl());
            System.out.println("The image " + imageBlobName + " product was successfully uploaded.");
            System.out.println("New cloud image url: " + blockBlobClient.getBlobUrl());
        } catch (Exception  e) {
            response.put("error","An error occurred while uploading the image "  + internalProductId + "/" + productId + " to the cloud.");
            response.put("message", e.toString());
        }
        return response;
    }

    private String getBlobName(String internalProductId, String productId){
        Integer imageBlobNumber = ecommImageProductsRepository.getMaxImageProductNumber(internalProductId, env.getProperty("integration.env.business.unit"));
        String blobName = internalProductId + "_" + productId.replace("/","_") + "_";
        if(imageBlobNumber == null){
            blobName += 1;
        } else {
            int auxImageNumber = imageBlobNumber + 1;
            blobName += auxImageNumber;
        }
        return blobName;
    }

    public HashMap saveUploadedImageInfo(String internalProductId, String productName, String productId, String blobName, String urlSavedImage){
        HashMap<String, String> response = new HashMap<>();
        try {
            Integer auxImageBlobNumber = ecommImageProductsRepository.getMaxImageProductNumber(internalProductId, env.getProperty("integration.env.business.unit"));
            int imageBlobNumber = auxImageBlobNumber == null ? 1 : auxImageBlobNumber + 1;
            EcommImageProducts ecommImageProducts = new EcommImageProducts();
            ecommImageProducts.setInternalProductId(internalProductId);
            ecommImageProducts.setProductName(productName);
            ecommImageProducts.setProductSearchId(productId);
            ecommImageProducts.setImageNumber(imageBlobNumber);
            ecommImageProducts.setBlobName(blobName);
            ecommImageProducts.setImageUrl(urlSavedImage);
            ecommImageProducts.setVegBusinessUnit(env.getProperty("integration.env.business.unit"));
            ecommImageProductsRepository.save(ecommImageProducts);
            response.put("ok","The scraped task finished successfully!");
            response.put("message","Image to " + internalProductId + "/" + productId + " saved in: " + urlSavedImage);
        } catch (RuntimeException e){
            response.put("error", "An error occurred when saving the info of scraped image: " + internalProductId + "/" + productId);
            response.put("message", e.toString());
        }
        return response;
    }
}
