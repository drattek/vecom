package com.vegusa.middleware.service;

import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.repository.local.*;
import com.vegusa.middleware.utils.MWUtils;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.util.Map;

@Service
public class ItemControlTableService {
    private final InterfaceRepository interfaceRepo;
    private final InterfaceItemsRepository interfaceItemsRepo;
    private final ProductAttributeRepository attributeRepo;
    private final ProductAttributeValueRepository attributeValuesRepo;
    private final CompanyRepository companyRepo;

    @Autowired
    public ItemControlTableService(InterfaceRepository interfaceRepo,
                                   InterfaceItemsRepository interfaceItemsRepo,
                                   ProductAttributeRepository attributeRepo,
                                   ProductAttributeValueRepository attributeValuesRepo,
                                   CompanyRepository companyRepo){
        this.interfaceRepo = interfaceRepo;
        this.interfaceItemsRepo = interfaceItemsRepo;
        this.attributeRepo = attributeRepo;
        this.attributeValuesRepo = attributeValuesRepo;
        this.companyRepo = companyRepo;
    }

    public Company getCompany(String dataAreaId) throws RuntimeException {
        return companyRepo.getCompany(dataAreaId);
    }

    public String updateControlTableInfo(String interfaceId, String dataAreaId) throws RuntimeException {
        JSONObject response = new JSONObject();
        Interface itf = interfaceRepo.getInterface(interfaceId, dataAreaId);
        if(itf == null){throw new RuntimeException("The interfaceId provided doesn't exist."); }
        InterfaceItems[] interfaceProducts = interfaceItemsRepo.getInterfaceProductsInfo(interfaceId, dataAreaId);
        for(InterfaceItems iProduct : interfaceProducts){
            try {
                Map<String, String> attributeValuesMap = getAttributeValuesMap(iProduct);
                saveControlTableProduct(iProduct, attributeValuesMap, interfaceId, dataAreaId);
                response.accumulate("ok", iProduct.getItemId() + " uploaded successfully.");
                System.out.println(iProduct.getItemId() + " saved successfully.");
            } catch (RuntimeException e){
                response.accumulate("error", "Error when uploading the product " + iProduct.getItemId());
                System.err.println("Error while uploading the product " + iProduct.getItemId());
            }
        }
        return response.toString();
    }

    private static Map<String, String> getAttributeValuesMap(InterfaceItems iProduct) {
        String auxAvailable = iProduct.getAvailable() == null ? null : iProduct.getAvailable().toString();
        String auxCost = iProduct.getAvailable() == null ? null : iProduct.getCost().toString();
        Map<String, String> attributeValuesMap = MWUtils.getControlTableAttributes();
        attributeValuesMap.put("ITEM_ID", iProduct.getItemId());
        attributeValuesMap.put("PRODUCT_NAME", iProduct.getProductName());
        attributeValuesMap.put("PART_NUMBER", iProduct.getPartNumber());
        attributeValuesMap.put("SHORT_DESCRIPTION", iProduct.getShortDescription());
        attributeValuesMap.put("BRAND_ID", iProduct.getBrand());
        attributeValuesMap.put("CATEGORY_ID", iProduct.getCategory());
        attributeValuesMap.put("WEIGHT", iProduct.getWeight());
        attributeValuesMap.put("UNIT_OF_MEASUREMENT", iProduct.getUnitOfMeasurement());
        attributeValuesMap.put("AVAILABLE", auxAvailable);
        attributeValuesMap.put("COST", auxCost);
        attributeValuesMap.put("LENGTH", iProduct.getLength());
        attributeValuesMap.put("HEIGHT", iProduct.getHeight());
        attributeValuesMap.put("WIDTH", iProduct.getWidth());
        attributeValuesMap.put("WARRANTY", iProduct.getWarranty());
        attributeValuesMap.put("CROSS_REFERENCES", iProduct.getCrossReferences());
        return attributeValuesMap;
    }

    private void saveControlTableProduct(InterfaceItems interfaceProduct, Map<String, String> attributeValuesMap, String interfaceId, String dataAreaId) throws RuntimeException {
        for (Map.Entry<String, String> attrValue : attributeValuesMap.entrySet()) {
            try {
                String itemId = interfaceProduct.getItemId();
                ProductAttributeValue auxAttrValue = attributeValuesRepo.getProductAttributeValue(attrValue.getKey(), itemId, interfaceId, dataAreaId);
                if(attrValue.getValue() != null) {
                    ProductAttribute productAttribute = attributeRepo.getProductAttribute(attrValue.getKey(), dataAreaId);
                    ProductAttributeValue attributeValue = auxAttrValue == null ? new ProductAttributeValue() : auxAttrValue;
                    attributeValue.setItemId(interfaceProduct.getItemId());
                    attributeValue.setInterfaceField(interfaceProduct.getInterfaceField());
                    attributeValue.setProductattribute(productAttribute);
                    attributeValue.setValue(attrValue.getValue());
                    attributeValue.setSkipNull("TRUE");
                    attributeValue.setCreatedAt(interfaceProduct.getCreatedAt());
                    attributeValue.setUpdatedAt(interfaceProduct.getUpdatedAt());
                    attributeValue.setCompany(interfaceProduct.getCompany());
                    attributeValue.setProductRefRec(interfaceProduct);
                    attributeValuesRepo.save(attributeValue);
                } else if(auxAttrValue != null){
                    attributeValuesRepo.deleteProductAttributeValue(attrValue.getKey(), itemId, interfaceId, dataAreaId);
                }
            } catch (RuntimeException e) {
                System.err.println("Error while saving ProductAttributeValue to " + interfaceProduct.getItemId() + "-" + attrValue.getKey());
            }
        }
    }
}
