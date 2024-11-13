package com.vegusa.middleware.service;

import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.repository.*;
import com.vegusa.msb.entity.DYNProduct;
import com.vegusa.msb.repository.DYNProductRepository;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.core.env.Environment;
import org.springframework.stereotype.Service;

import java.time.LocalDateTime;
import java.time.ZoneId;
import java.time.ZonedDateTime;
import java.util.Date;
import java.util.Objects;

@Service
public class InterfaceInfoService {
    //Dynamics ERP-Repository
    private final DYNProductRepository dynProductRepo;
    //Middleware-Repository
    private final ItemExtraInfoRepository itemExtraInfoRepo;
    private final InterfaceRepository interfaceRepo;
    private final InterfaceItemsRepository interfaceItemsRepo;
    private final SystemParameterRepository sysParameterRepo;
    private final CompanyRepository companyRepo;
    private final Environment env;

    @Autowired
    public InterfaceInfoService(DYNProductRepository dynProductRepo,
                                ItemExtraInfoRepository itemExtraInfoRepo,
                                InterfaceRepository interfaceRepo,
                                InterfaceItemsRepository interfaceItemsRepo,
                                SystemParameterRepository sysParameterRepo,
                                CompanyRepository companyRepo,
                                Environment env){
        this.dynProductRepo = dynProductRepo;
        this.itemExtraInfoRepo = itemExtraInfoRepo;
        this.interfaceRepo = interfaceRepo;
        this.interfaceItemsRepo = interfaceItemsRepo;
        this.sysParameterRepo = sysParameterRepo;
        this.companyRepo = companyRepo;
        this.env = env;
    }

    public Company getCompany(String dataAreaId) throws RuntimeException {
        return companyRepo.getCompany(dataAreaId);
    }

    public String updateDYNInterfaceInfo(String interfaceId, String dataAreaId, Company company) throws RuntimeException {
        JSONObject response = new JSONObject();
        Interface itf = interfaceRepo.getInterface(interfaceId, dataAreaId);
        if(itf == null){throw new RuntimeException("The interfaceId provided doesn't exist."); }
        DYNProduct[] dynProducts = dynProductRepo.getDYNProducts();
        for (DYNProduct dynProduct : dynProducts) {
            try {
                ZonedDateTime zdt = ZonedDateTime.of(LocalDateTime.now(), ZoneId.of(sysParameterRepo
                        .getSystemParameter("ZONE_ID").getStrValue()));
                Date date = Date.from(zdt.toInstant());
                InterfaceItems auxIProduct = interfaceItemsRepo.getInterfaceProduct(dynProduct.getArticulo(), interfaceId, dataAreaId);
                InterfaceItems product = auxIProduct == null ? new InterfaceItems() : auxIProduct;
                product.setItemId(dynProduct.getArticulo());
                product.setProductName(dynProduct.getDescripcion());
                product.setShortDescription(dynProduct.getDescripcion());
                product.setPartNumber(dynProduct.getNumParte());
                product.setCategory(dynProduct.getCateogria());
                product.setBrand(dynProduct.getMarca());
                product.setAvailable(dynProduct.getDisponible().doubleValue());
                product.setCost(dynProduct.getCosto().doubleValue());
                product.setUpdatedAt(date);
                product.setSkipNull("TRUE");
                product.setInterfaceField(itf);
                product.setCompany(company);
                if (auxIProduct == null) {
                    product.setCreatedAt(date);
                }
                interfaceItemsRepo.save(product);
                response.accumulate("ok", dynProduct.getArticulo() + " updated successfully.");
                System.out.println(dynProduct.getArticulo() + " updated successfully.");
            } catch (RuntimeException e) {
                response.accumulate("error", "Error when updating the product " + dynProduct.getArticulo());
                System.err.println("Error while uploading the product " + dynProduct.getArticulo());
            }
        }
        return response.toString();
    }

    public String updateInterfaceInfo(String interfaceId, String dataAreaId, Company company) throws RuntimeException {
        JSONObject response = new JSONObject();
        Interface itf = interfaceRepo.getInterface(interfaceId, dataAreaId);
        if(itf == null){throw new RuntimeException("The interfaceId provided doesn't exist."); }
        ItemExtraInfo[] scrapedProducts = itemExtraInfoRepo.getItemExtraInfo(interfaceId, dataAreaId);
        for (ItemExtraInfo scrapedProduct : scrapedProducts) {
            try {
                ZonedDateTime zdt = ZonedDateTime.of(LocalDateTime.now(), ZoneId.of(sysParameterRepo
                        .getSystemParameter("ZONE_ID").getStrValue()));
                Date date = Date.from(zdt.toInstant());
                InterfaceItems auxProduct = interfaceItemsRepo.getInterfaceProduct(scrapedProduct.getItemId(), interfaceId, dataAreaId);
                InterfaceItems product = auxProduct == null ? new InterfaceItems() : auxProduct;
                product.setItemId(scrapedProduct.getItemId());
                product.setProductName(scrapedProduct.getItemName());
                product.setShortDescription(scrapedProduct.getShortDescription());
                product.setPartNumber(scrapedProduct.getPartNumberSearched());
                product.setCategory(scrapedProduct.getCategory());
                product.setWeight(scrapedProduct.getWeight());
                product.setUnitOfMeasurement(scrapedProduct.getUnitOfMeasure());
                product.setUpdatedAt(date);
                product.setSkipNull("TRUE");
                product.setInterfaceField(itf);
                product.setCompany(company);
                if (auxProduct == null) {
                    product.setCreatedAt(date);
                }
                interfaceItemsRepo.save(product);
                response.accumulate("ok", scrapedProduct.getItemId() + " updated successfully.");
                System.out.println(scrapedProduct.getItemId() + " updated successfully.");
            } catch (RuntimeException e) {
                response.accumulate("error", "Error when updating the product " + scrapedProduct.getItemId() + " " + e.getMessage());
                System.err.println(scrapedProduct.getItemId() + " error while saving.");
            }
        }
        return response.toString();
    }
}
