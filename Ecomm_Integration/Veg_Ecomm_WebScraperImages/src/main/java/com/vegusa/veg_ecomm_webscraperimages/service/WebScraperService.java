package com.vegusa.veg_ecomm_webscraperimages.service;

import com.azure.storage.blob.BlobContainerClient;
import com.azure.storage.blob.BlobContainerClientBuilder;
import com.azure.storage.blob.specialized.BlockBlobClient;
import com.vegusa.veg_ecomm_webscraperimages.entity.EcomPartCrossReference;
import com.vegusa.veg_ecomm_webscraperimages.entity.EcomScrapedAdditionalInfo;
import com.vegusa.veg_ecomm_webscraperimages.entity.ScrapedImage;
import com.vegusa.veg_ecomm_webscraperimages.repository.EcomPartCrossReferenceRepository;
import com.vegusa.veg_ecomm_webscraperimages.repository.EcomImageProductsRepository;
import com.vegusa.veg_ecomm_webscraperimages.repository.EcomScrapedAdditionalInfoRepository;
import com.vegusa.veg_ecomm_webscraperimages.repository.SynchronizedProductRepository;
import org.json.JSONArray;
import org.json.JSONObject;
import org.jsoup.Jsoup;
import org.jsoup.nodes.Document;
import org.jsoup.nodes.Element;
import org.jsoup.select.Elements;
import org.openqa.selenium.By;
import org.openqa.selenium.WebDriver;
import org.openqa.selenium.WebElement;
import org.openqa.selenium.chrome.ChromeDriver;
import org.openqa.selenium.chrome.ChromeOptions;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.core.env.Environment;
import org.springframework.http.HttpHeaders;
import org.springframework.stereotype.Service;
import org.springframework.web.reactive.function.client.WebClient;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.net.URL;
import java.time.LocalDateTime;
import java.time.ZoneId;
import java.time.ZonedDateTime;
import java.util.*;

@Service
public class WebScraperService {
    private final Environment env;
    private final EcomImageProductsRepository ecommImageProductsRepository;
    private final EcomScrapedAdditionalInfoRepository ecommProductsInfoRepository;
    private final EcomPartCrossReferenceRepository ecomPartCrossReferenceRepository;
    private final SynchronizedProductRepository syncProducts;
    private final WebClient webClient;

    @Autowired
    public WebScraperService(EcomImageProductsRepository ecommImageProductsRepository,
                             EcomScrapedAdditionalInfoRepository ecommProductsInfoRepository,
                             EcomPartCrossReferenceRepository ecomPartCrossReferenceRepository,
                             SynchronizedProductRepository syncProducts,
                             WebClient webClient,
                             Environment env){
        this.ecommImageProductsRepository = ecommImageProductsRepository;
        this.ecommProductsInfoRepository = ecommProductsInfoRepository;
        this.ecomPartCrossReferenceRepository = ecomPartCrossReferenceRepository;
        this.syncProducts = syncProducts;
        this.webClient = webClient;
        this.env = env;
    }

    public WebDriver loginPageTVH(String userEmail, String userPass) {
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

    public WebDriver getUCADriverLoginPage(String userEmail, String userPass){
        ChromeOptions chromeOptions = new ChromeOptions();
        chromeOptions.addArguments("--headless");
        chromeOptions.addArguments("window-size=1200,1100");
        //Create web driver
        WebDriver driver = new ChromeDriver(chromeOptions);
        //LOGIN PAGE
        driver.get(env.getProperty("integration.env.uca-login-page"));
        //Find and set username
        WebElement username = driver.findElement(By.id("username"));
        username.sendKeys(userEmail);
        //Find and set password
        WebElement password = driver.findElement(By.id("password"));
        password.sendKeys(userPass);
        //Find and click login button
        WebElement loginButton = driver.findElement(By.id("btnSubmit"));
        loginButton.click();
        driver.get(env.getProperty("integration.env.uca-verify-access-page"));
        System.out.println("Ingresa el código de verificación: ");
        Scanner scanner = new Scanner(System.in);
        String code = scanner.nextLine();
        WebElement verificationCode = driver.findElement(By.id("VerificationCode"));
        verificationCode.sendKeys(code);
        WebElement submitBtn = driver.findElement(By.id("btnSubmit"));
        submitBtn.click();
        return driver;
    }

    public HashMap<String, String> scraperTVHPage(String internalProductId, String productId, WebDriver driver, String aim) {
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
            fillTVHScrapedInfo(response, driver, aim);
            System.out.println("The scraper to product " + productId + " ended.");
        } catch(RuntimeException e) {
            response.put("error", "An error occurred while scraping the image of: " + internalProductId + "/" + productId);
            response.put("message", e.toString());
        }
        return response;
    }

    private void fillTVHScrapedInfo(HashMap response, WebDriver driver, String aim) throws RuntimeException {
        //SCRAPE PAGE
        //Retrieve the page source from Selenium
        String pageSource = driver.getPageSource();
        //Parse the page source with jsoup
        Document page = Jsoup.parse(pageSource);
        switch (aim)
        {
            case "images":
                Element image = page.getElementById("IIProductImage");
                response.put("image", image.attr("data-image-list-x").substring(2, image.attr("data-image-list-x").length() - 2));
                System.out.println("Found image: " + image.attr("data-image-list-x").substring(2, image.attr("data-image-list-x").length() - 2));
                break;
            case "productInfo":
                Element lblItemDesc = page.getElementById("ctl00_cphContent_ItemInformation1_lbItemDescription");
                Element lblItemNum = page.getElementById("ctl00_cphContent_ItemInformation1_lbItemNumber");
                Element weight = page.getElementById("ctl00_cphContent_ItemInformation1_lbWeight");
                Element measure = page.getElementById("ctl00_cphContent_ItemInformation1_lbUnitOfMeasure");
                Element crossReference = page.getElementById("ctl00_cphContent_ItemInformation1_divCrossScroll");
                response.put("shortDescription", lblItemDesc.text() + " " + lblItemNum.text());
                response.put("weight", weight.text());
                response.put("measure", measure.text());
                response.put("crossReference", crossReference.text());
                break;
            default:
                // default statement
                break;
        }
    }

    public List<String> scraperUCAPage(String internalProductId, String productId, WebDriver driver) {
        List<String> response = new ArrayList<String>();
        System.out.println("Enter to scrape method: " + internalProductId);
        try {
            //HOME PAGE
            driver.get(env.getProperty("integration.env.uca-home-page"));
            System.out.println("Charging home page...");
            Thread.sleep(7000);
            //Search text box
            WebElement searchText = driver.findElement(By.id("headerSearchPartNumber"));
            searchText.sendKeys(productId);
            //Search button
            WebElement searchButton = driver.findElement(By.id("btnId"));
            searchButton.click();
            System.out.println("Charging part number info...");
            Thread.sleep(7000);
            if(validateScrapField(driver)){
                System.out.println("Find multiple results!");
                WebElement partInfo = driver.findElement(By.className("part-info-component__img-container"));
                partInfo.click();
                System.out.println("Charging the first result...");
                Thread.sleep(   7000);
            }
            //SCRAPE PAGE
            //Retrieve the page source from Selenium
            String pageSource = driver.getPageSource();
            //Parse the page source with jsoup
            Document page = Jsoup.parse(pageSource);
            Element partNumberFound = page.getElementById("part-detail-part-number");
            String auxNumParte = partNumberFound != null ? partNumberFound.text() : "";
            System.out.println("Part number found: " + auxNumParte);
            response.add(auxNumParte);
            Elements images = page.getElementsByClass("carousel-item");
            for (Element image : images) {
                try {
                    String auxImage = image.select("img").attr("src");
                    System.out.println("Image found: " + auxImage);
                    response.add(auxImage);
                }catch(RuntimeException e){
                    System.err.println("An error occurred while scraping the image.");
                }
            }
            System.out.println("The scrape to product " + productId + " ended.");
        } catch(RuntimeException | InterruptedException e) {
            System.err.println("Error en el scrape: " + e.getMessage());
            throw new RuntimeException("An error occurred while scraping the image of: " + internalProductId + "/" + productId + " " + e.getMessage());
        }
        return response;
    }

    private boolean validateScrapField(WebDriver driver){
        boolean response = true;
        try {
            driver.findElement(By.id("categoryParts"));
        } catch (RuntimeException e){
            response = false;
        }
        return response;
    }

    public HashMap<String, String> uploadImageToAzure(String internalProductId, String productId, String url) {
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

    public HashMap<String, String> saveUploadedImageInfo(String internalProductId, String productName, String productId, String blobName, String urlSavedImage, String partNumberFound){
        HashMap<String, String> response = new HashMap<>();
        try {
            Integer auxImageBlobNumber = ecommImageProductsRepository.getMaxImageProductNumber(internalProductId, env.getProperty("integration.env.business.unit"));
            int imageBlobNumber = auxImageBlobNumber == null ? 1 : auxImageBlobNumber + 1;
            ZonedDateTime zdt = ZonedDateTime.of(LocalDateTime.now(), ZoneId.of("America/Mexico_City"));
            Date date = Date.from(zdt.toInstant());
            ScrapedImage ecommImageProducts = new ScrapedImage();
            ecommImageProducts.setInternalProductId(internalProductId);
            ecommImageProducts.setProductName(productName);
            ecommImageProducts.setPartNumberSearched(productId);
         //   ecommImageProducts.setNewPartNumberFound(partNumberFound);
            ecommImageProducts.setImageNumber(imageBlobNumber);
            ecommImageProducts.setBlobName(blobName);
            ecommImageProducts.setImageUrl(urlSavedImage);
            ecommImageProducts.setBusinessUnit(env.getProperty("integration.env.business.unit"));
            ecommImageProducts.setCreatedAt(date);
            ecommImageProducts.setUpdatedAt(date);
            ecommImageProducts.setDownloadPortal(partNumberFound);
            ecommImageProductsRepository.save(ecommImageProducts);
            response.put("ok","The scraped task finished successfully!");
            response.put("message","Image to " + internalProductId + "/" + productId + " saved in: " + urlSavedImage);
        } catch (RuntimeException e){
            response.put("error", "An error occurred when saving the info of scraped image: " + internalProductId + "/" + productId);
            response.put("message", e.toString());
        }
        return response;
    }

    public HashMap<String, String> saveScrapedProductInfo(String internalProductId, String productName, String productId, HashMap scrapedInfo){
        HashMap<String, String> response = new HashMap<>();
        try{
            ZonedDateTime zdt = ZonedDateTime.of(LocalDateTime.now(), ZoneId.of("America/Mexico_City"));
            Date date = Date.from(zdt.toInstant());
            EcomScrapedAdditionalInfo vegEcommScrapedAdditionalInfo = new EcomScrapedAdditionalInfo();
            vegEcommScrapedAdditionalInfo.setInternalProductId(internalProductId);
            vegEcommScrapedAdditionalInfo.setProductName(productName);
            vegEcommScrapedAdditionalInfo.setProductSearchId(productId);
            vegEcommScrapedAdditionalInfo.setShortDescription(scrapedInfo.get("shortDescription").toString());
            vegEcommScrapedAdditionalInfo.setWeight(scrapedInfo.get("weight").toString());
            vegEcommScrapedAdditionalInfo.setUnitOfMeasure(scrapedInfo.get("measure").toString());
            vegEcommScrapedAdditionalInfo.setCrossReference(scrapedInfo.get("crossReference").toString());
            vegEcommScrapedAdditionalInfo.setVegBusinessUnit(env.getProperty("integration.env.business.unit"));
            vegEcommScrapedAdditionalInfo.setCreatedAt(date);
            vegEcommScrapedAdditionalInfo.setUpdatedAt(date);
            vegEcommScrapedAdditionalInfo.setDownloadPortal("TVH");
            ecommProductsInfoRepository.save(vegEcommScrapedAdditionalInfo);
            response.put("ok","The scraped info task finished successfully to " + internalProductId + "/" + productId );
        }catch (RuntimeException e){
            response.put("error", "An error occurred when saving the scraped product info: " + internalProductId + "/" + productId);
            response.put("message", e.toString());
        }
        return response;
    }

    public String getLogisNextInfo(String productId) throws RuntimeException {
        HttpHeaders headers = new HttpHeaders();
        headers.add("Content-Type", "application/json");
        headers.add("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJjaWQiOiIwMkZtSm9iWHp0cE1PYmt6IiwiaXBzIjoiMCIsImlwZSI6IjQyOTQ5NjcyOTUiLCJha2kiOiIxNDIiLCJha3YiOiJZTDJVRVR3dmpudWsxN1NmSVc4OEF3Mm5VaWNaSlZaY24wWmNHRzdYIiwiYWtuIjoiTUxBX1ZlZ3VzYSIsIm1pZCI6IjEiLCJhaWQiOiIyMDAxIiwidWlkIjoiNDc3MDAiLCJhb2kiOlsiQ3JlZGl0TWVtb19TZWFyY2hCeURhdGVSYW5nZSIsIkNyZWRpdE1lbW9fU2VhcmNoQnlJZGVudGlmaWVyIiwiSW52b2ljZV9TZWFyY2hCeURhdGVSYW5nZSIsIkludm9pY2VfU2VhcmNoQnlJZGVudGlmaWVyIiwiT3JkZXJfR2V0QmFja29yZGVycyIsIk9yZGVyX0dldFJlY2VudGx5TW9kaWZpZWRCYWNrb3JkZXJzIiwiT3JkZXJfR2V0U2hpcHBpbmdPcHRpb25zIiwiT3JkZXJfR2V0U2hpcHBpbmdPcHRpb25zRm9yRmFjaW5nV2FyZWhvdXNlIiwiT3JkZXJfU2VhcmNoQmFja29yZGVycyIsIk9yZGVyX1NlYXJjaE9yZGVyQnlQb09yT3JkZXJOdW1iZXIiLCJPcmRlcl9TdWJtaXRPcmRlciIsIlBhcnRzX0dldFBhcnREZXRhaWxCeU9lbVBhcnRDb2RlIiwiUGFydHNfR2V0UGFydERldGFpbEJ5UGFydElkIiwiUGFydHNfR2V0UGFydFNlYXJjaFJlc3VsdHMiXSwiYWFpIjpbIjE2MDM0IiwiMTYxMjUiLCIxNjQ2MyIsIjE4NjQ2IiwiMTg2NDciXSwibmJmIjoxNzIwMTExOTc5LCJleHAiOjE3MjAxMTM3NzksImlhdCI6MTcyMDExMTk3OX0.a0QtiqzhSVJ8-BNLoS_O6PAaM_ivjQz3E0XDRJC2MOQ");
        return webClient.get()
                .uri("https://solutionsapi-qa.logisnextdealers.com/rest/V1/Parts/GetPartDetailsByOemPartCode?oemPartCode=" + productId)
                .headers(h -> h.addAll(headers))
                .retrieve()
                .bodyToMono(String.class)
                .block();
    }

    public void saveLogisNextInfo(String internalProductId, String productName, String productId, String productInfo) throws RuntimeException {
        ZonedDateTime zdt = ZonedDateTime.of(LocalDateTime.now(), ZoneId.of("America/Mexico_City"));
        Date date = Date.from(zdt.toInstant());
        JSONObject productsInfo = new JSONObject(productInfo);
        JSONObject productValidation = productsInfo.getJSONObject("Validation");
        JSONArray crossReferences = productsInfo.getJSONArray("PartCrossReferences");
        EcomScrapedAdditionalInfo vegEcommScrapedAdditionalInfo = new EcomScrapedAdditionalInfo();
        vegEcommScrapedAdditionalInfo.setInternalProductId(internalProductId);
        vegEcommScrapedAdditionalInfo.setProductName(productName);
        vegEcommScrapedAdditionalInfo.setProductSearchId(productId);
        vegEcommScrapedAdditionalInfo.setShortDescription(productsInfo.get("PartDescription").toString());
        vegEcommScrapedAdditionalInfo.setWeight(productsInfo.get("Weight").toString());
        vegEcommScrapedAdditionalInfo.setUnitOfMeasure(productsInfo.get("UnitOfMeasure").toString());
        vegEcommScrapedAdditionalInfo.setValidationReason(productValidation.get("ValidationReason").toString());
        vegEcommScrapedAdditionalInfo.setContractPrice(productsInfo.get("ContractPrice").toString());
        vegEcommScrapedAdditionalInfo.setImageUrl(productsInfo.get("ImageUrl").toString());
        vegEcommScrapedAdditionalInfo.setOemId(productsInfo.get("OemId").toString());
        vegEcommScrapedAdditionalInfo.setUcaId(productsInfo.get("Id").toString());
        vegEcommScrapedAdditionalInfo.setFleetListPrice(productsInfo.get("FleetListPrice").toString());
        vegEcommScrapedAdditionalInfo.setDealerNetPrice(productsInfo.get("DealerNetPrice").toString());
        vegEcommScrapedAdditionalInfo.setCompany(productsInfo.get("Company").toString());
        vegEcommScrapedAdditionalInfo.setMarketingDescription(productsInfo.get("MarketingDescription").toString());
        vegEcommScrapedAdditionalInfo.setItemCategory(productsInfo.get("ItemCategory").toString());
        vegEcommScrapedAdditionalInfo.setVegBusinessUnit(env.getProperty("integration.env.business.unit"));
        vegEcommScrapedAdditionalInfo.setCreatedAt(date);
        vegEcommScrapedAdditionalInfo.setUpdatedAt(date);
        vegEcommScrapedAdditionalInfo.setDownloadPortal("UCA");
        ecommProductsInfoRepository.save(vegEcommScrapedAdditionalInfo);
        this.savePartCrossReferences(internalProductId, date, crossReferences);
    }

    private void savePartCrossReferences(String internalProductId, Date date, JSONArray crossReferences) throws RuntimeException {
        for (int it = 0; it < crossReferences.length(); it++) {
            try {
                JSONObject reference = crossReferences.getJSONObject(it);
                EcomPartCrossReference vegEcomPartCrossReference = new EcomPartCrossReference();
                vegEcomPartCrossReference.setInternalProductId(internalProductId);
                vegEcomPartCrossReference.setPartNumber(reference.get("PartNumber").toString());
                vegEcomPartCrossReference.setOemId(reference.get("OemId").toString());
                vegEcomPartCrossReference.setOemDescription(reference.get("OemDescription").toString());
                vegEcomPartCrossReference.setCrossOemListPrice(reference.get("CrossOemListPrice").toString());
                vegEcomPartCrossReference.setOriginalPartNumber(reference.get("OriginalPartId").toString());
                vegEcomPartCrossReference.setVegBusinessUnit(env.getProperty("integration.env.business.unit"));
                vegEcomPartCrossReference.setCreatedAt(date);
                vegEcomPartCrossReference.setUpdatedAt(date);
                vegEcomPartCrossReference.setDownloadPortal("UCA");
                ecomPartCrossReferenceRepository.save(vegEcomPartCrossReference);
            } catch (RuntimeException e){
                System.err.println("An error occurred while saving cross reference of " + internalProductId);
            }
        }
    }

    public void fixCrossReferences(){
        EcomScrapedAdditionalInfo[] ecomScrapedAdditionalInfo = ecommProductsInfoRepository.getAdditionalInfo("TVH");
        String internalProductId, originalPartId, auxPartNumber, auxOEMDescription, auxOriginalChain;
        int auxChainLength, auxCharIndex;
        for(int it = 0; it < ecomScrapedAdditionalInfo.length; it++){
            internalProductId = ecomScrapedAdditionalInfo[it].getInternalProductId();
            originalPartId = ecomScrapedAdditionalInfo[it].getProductSearchId();
            auxOriginalChain = ecomScrapedAdditionalInfo[it].getCrossReference();
            auxChainLength = auxOriginalChain.length();
            auxCharIndex = auxOriginalChain.indexOf("/");
            do{
                auxOriginalChain = auxOriginalChain.substring(auxCharIndex + 2, auxChainLength);
                auxChainLength = auxOriginalChain.length();
                auxCharIndex = auxOriginalChain.indexOf("[");
                auxPartNumber = auxOriginalChain.substring(0, auxCharIndex - 1);
                auxOriginalChain = auxOriginalChain.substring(auxCharIndex + 1, auxChainLength);
                auxChainLength = auxOriginalChain.length();
                auxCharIndex = auxOriginalChain.indexOf("]");
                auxOEMDescription = auxOriginalChain.substring(0, auxCharIndex);
                auxOriginalChain = auxOriginalChain.substring(auxCharIndex, auxChainLength);
                auxChainLength = auxOriginalChain.length();
                createCrossReference(internalProductId, originalPartId, auxPartNumber, auxOEMDescription);
                auxCharIndex = auxOriginalChain.indexOf("/");
            } while(auxCharIndex != -1);
            System.out.println("Processed info to: " + ecomScrapedAdditionalInfo[it].getInternalProductId());
        }
    }

    private void createCrossReference(String internalProductId, String originalPartId, String auxPartNumber, String auxOEMDescription){
        try {
            ZonedDateTime zdt = ZonedDateTime.of(LocalDateTime.now(), ZoneId.of("America/Mexico_City"));
            Date date = Date.from(zdt.toInstant());
            EcomPartCrossReference vegEcomPartCrossReference = new EcomPartCrossReference();
            vegEcomPartCrossReference.setInternalProductId(internalProductId);
            vegEcomPartCrossReference.setPartNumber(auxPartNumber);
            vegEcomPartCrossReference.setOemDescription(auxOEMDescription);
            vegEcomPartCrossReference.setOriginalPartNumber(originalPartId);
            vegEcomPartCrossReference.setVegBusinessUnit(env.getProperty("integration.env.business.unit"));
            vegEcomPartCrossReference.setCreatedAt(date);
            vegEcomPartCrossReference.setUpdatedAt(date);
            vegEcomPartCrossReference.setDownloadPortal("TVH");
            ecomPartCrossReferenceRepository.save(vegEcomPartCrossReference);
            System.out.println("Cross reference  saved: " + internalProductId + " / " + auxPartNumber);
        } catch (RuntimeException e){
            System.err.println("An error occurred while saving cross reference of " + internalProductId);
        }
    }

    public void addFoundedPartNumberColumn(){
        EcomScrapedAdditionalInfo[] additionalInfo = ecommProductsInfoRepository.getAdditionalInfo("TVH");
        String auxShortDescription, newValue;
        int auxIndex, auxSize;
        for (EcomScrapedAdditionalInfo ecomScrapedAdditionalInfo : additionalInfo) {
            auxShortDescription = ecomScrapedAdditionalInfo.getShortDescription();
            auxIndex = auxShortDescription.lastIndexOf(" / ") + 3;
            auxSize = auxShortDescription.length();
            newValue = auxShortDescription.substring(auxIndex, auxSize);
            ecomScrapedAdditionalInfo.setPartNumber(newValue);
            ecommProductsInfoRepository.save(ecomScrapedAdditionalInfo);
            System.out.println("End work to " + ecomScrapedAdditionalInfo.getInternalProductId());
        }
    }



}
