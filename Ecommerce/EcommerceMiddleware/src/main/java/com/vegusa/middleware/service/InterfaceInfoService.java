package com.vegusa.middleware.service;

import com.vegusa.middleware.entity.Company;
import com.vegusa.middleware.entity.InterfaceDS;
import com.vegusa.middleware.entity.InterfaceProduct;
import com.vegusa.middleware.entity.ProductAdditionalInfo;
import com.vegusa.msb.entity.MSBProduct;
import com.vegusa.msb.repository.MSBProductRepository;
import com.vegusa.middleware.repository.CompanyRepository;
import com.vegusa.middleware.repository.InterfaceDSRepository;
import com.vegusa.middleware.repository.InterfaceProductRepository;
import com.vegusa.middleware.repository.ProductAdditionalInfoRepository;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.time.LocalDateTime;
import java.time.ZoneId;
import java.time.ZonedDateTime;
import java.util.Date;
import java.util.HashMap;
import java.util.Objects;

@Service
public class InterfaceInfoService {
    //MSB-Repository
    private final MSBProductRepository dynProducts;
    //Middleware-Repository
    private final CompanyRepository companyRepo;
    private final InterfaceDSRepository interfaces;
    private final InterfaceProductRepository interfaceProducts;
    private final ProductAdditionalInfoRepository additionalProducts;

    @Autowired
    public InterfaceInfoService(MSBProductRepository dynProducts,
                                CompanyRepository companyRepo,
                                InterfaceDSRepository interfaces,
                                InterfaceProductRepository interfaceProducts,
                                ProductAdditionalInfoRepository additionalProducts){
        this.dynProducts = dynProducts;
        this.companyRepo = companyRepo;
        this.interfaces = interfaces;
        this.interfaceProducts = interfaceProducts;
        this.additionalProducts = additionalProducts;
    }

    public Company getCompany(String dataAreaId) throws RuntimeException {
        return companyRepo.getCompany(dataAreaId);
    }

    public String updateDYNInterfaceInfo(String dataAreaId, String interfaceId) throws RuntimeException{
        JSONObject response = new JSONObject();
        InterfaceDS interfaceDS = interfaces.getInterface(interfaceId);
        if(!Objects.equals(interfaceDS.getCompany().getId().getDataAreaId(), dataAreaId)){
            throw new RuntimeException("The dataAreaId or interfaceId contains invalid information.");
        }
        MSBProduct[] dataSourceInfo = dynProducts.getDYNProducts();
        Company company = companyRepo.getCompany(dataAreaId);
        if(company != null) {
            for (MSBProduct dsProduct : dataSourceInfo) {
                try {
                    HashMap<String, String> uploadedInfo = new HashMap<>();
                    ZonedDateTime zdt = ZonedDateTime.of(LocalDateTime.now(), ZoneId.of("America/Mexico_City"));
                    Date date = Date.from(zdt.toInstant());
                    InterfaceProduct auxProduct = interfaceProducts.getInterfaceProduct(dsProduct.getArticulo(), interfaceId, dataAreaId);
                    InterfaceProduct product = auxProduct == null ? new InterfaceProduct() : auxProduct;
                    product.setItemId(dsProduct.getArticulo());
                    product.setProductName(dsProduct.getDescripcion());
                    product.setShortDescription(dsProduct.getDescripcion());
                    product.setPartNumber(dsProduct.getNumParte());
                    product.setCategory(dsProduct.getCateogria());
                    product.setBrand(dsProduct.getMarca());
                    product.setAvailable(dsProduct.getDisponible().doubleValue());
                    product.setCost(dsProduct.getCosto().doubleValue());
                    product.setUpdatedAt(date);
                    product.setSkipNull("TRUE");
                    product.setInterfaceField(interfaceDS);
                    product.setCompany(company);
                    if (auxProduct == null) {
                        product.setCreatedAt(date);
                    }
                    interfaceProducts.save(product);
                    uploadedInfo.put("ok", dsProduct.getArticulo() + " uploaded successfully.");
                    response.accumulate("uploaded", uploadedInfo);
                    System.out.println(dsProduct.getArticulo() + " saved successfully.");
                } catch (RuntimeException e) {
                    HashMap<String, String> errorInfo = new HashMap<>();
                    errorInfo.put("error", "Error when updating the product " + dsProduct.getArticulo());
                    response.accumulate("noUpdated", errorInfo);
                    System.err.println("Error while uploading the product " + dsProduct.getArticulo());
                }
            }
            return response.toString();
        }else{
            throw new RuntimeException("The dataAreaId contains invalid information.");
        }
    }

    public String updateInterfaceInfo(String dataAreaId, String interfaceId) throws RuntimeException{
        JSONObject response = new JSONObject();
        ProductAdditionalInfo[] dataSourceInfo = additionalProducts.getProductsAdditionalInfo(interfaceId, dataAreaId);
        Company company = companyRepo.getCompany(dataAreaId);
        InterfaceDS interfaceDS = interfaces.getInterface(interfaceId);
        if(!Objects.equals(interfaceDS.getCompany().getId().getDataAreaId(), dataAreaId)){
            throw new RuntimeException("The dataAreaId or interfaceId contains invalid information.");
        }
        if(company != null) {
            for (ProductAdditionalInfo dsProduct : dataSourceInfo) {
                try {
                    HashMap<String, String> updatedInfo = new HashMap<>();
                    ZonedDateTime zdt = ZonedDateTime.of(LocalDateTime.now(), ZoneId.of("America/Mexico_City"));
                    Date date = Date.from(zdt.toInstant());
                    InterfaceProduct auxProduct = interfaceProducts.getInterfaceProduct(dsProduct.getInternalProductId(), interfaceId, dataAreaId);
                    InterfaceProduct product = auxProduct == null ? new InterfaceProduct() : auxProduct;
                    product.setItemId(dsProduct.getInternalProductId());
                    product.setProductName(dsProduct.getProductName());
                    product.setShortDescription(dsProduct.getShortDescription());
                    product.setPartNumber(dsProduct.getProductSearchId());
                    product.setCategory(dsProduct.getItemCategory());
                    product.setWeight(dsProduct.getWeight());
                    product.setUnitOfMeasurement(dsProduct.getUnitOfMeasure());
                    product.setUpdatedAt(date);
                    product.setSkipNull("TRUE");
                    product.setInterfaceField(interfaceDS);
                    product.setCompany(company);
                    if (auxProduct == null) {
                        product.setCreatedAt(date);
                    }
                    interfaceProducts.save(product);
                    updatedInfo.put("ok", dsProduct.getInternalProductId() + " updated successfully.");
                    response.accumulate("updated", updatedInfo);
                    System.out.println(dsProduct.getInternalProductId() + " saved successfully.");
                } catch (RuntimeException e) {
                    HashMap<String, String> errorInfo = new HashMap<>();
                    errorInfo.put("error", "Error when updating the product " + dsProduct.getInternalProductId());
                    response.accumulate("noUpdated", errorInfo);
                    System.err.println(dsProduct.getInternalProductId() + " error while saving.");
                }
            }
            return response.toString();
        }else{
            throw new RuntimeException("The dataAreaId contains invalid information.");
        }
    }
}
