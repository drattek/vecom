package com.vegusa.veg_mv_integration_midd.msb.service;

import com.vegusa.veg_mv_integration_midd.msb.entity.ECOMProduct;
import com.vegusa.veg_mv_integration_midd.msb.repository.ProductRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.*;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.CompanyRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.InterfaceDSRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.InterfaceProductRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.ProductAdditionalInfoRepository;
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
public class InterfaceInformationService {
    //MSB-Repository
    private final ProductRepository dynProducts;
    //Middleware-Repository
    private final CompanyRepository companies;
    private final InterfaceDSRepository interfaces;
    private final InterfaceProductRepository interfaceProducts;
    private final ProductAdditionalInfoRepository additionalProducts;

    @Autowired
    public InterfaceInformationService(ProductRepository dynProducts,
                                       CompanyRepository companies,
                                       InterfaceDSRepository interfaces,
                                       InterfaceProductRepository interfaceProducts,
                                       ProductAdditionalInfoRepository additionalProducts){
        this.dynProducts = dynProducts;
        this.companies = companies;
        this.interfaces = interfaces;
        this.interfaceProducts = interfaceProducts;
        this.additionalProducts = additionalProducts;
    }

    public String updateDYNInterfaceInfo(String dataAreaId, String interfaceId) throws RuntimeException{
        JSONObject response = new JSONObject();
        InterfaceDS interfaceDS = interfaces.getInterface(interfaceId);
        if(!Objects.equals(interfaceDS.getCompany().getId().getDataAreaId(), dataAreaId)){
            throw new RuntimeException("The dataAreaId or interfaceId contains invalid information.");
        }
        ECOMProduct[] dataSourceInfo = dynProducts.getDYNProducts();
        Company company = companies.getCompany(dataAreaId);
        if(company != null) {
            for (ECOMProduct dsProduct : dataSourceInfo) {
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
        Company company = companies.getCompany(dataAreaId);
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
