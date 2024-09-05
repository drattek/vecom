package com.vegusa.middleware.service;

import com.vegusa.middleware.entity.InterfaceDS;
import com.vegusa.middleware.entity.InterfaceProduct;
import com.vegusa.middleware.entity.ProductAttribute;
import com.vegusa.middleware.entity.ProductAttributeValue;
import com.vegusa.middleware.repository.InterfaceDSRepository;
import com.vegusa.middleware.repository.InterfaceProductRepository;
import com.vegusa.middleware.repository.ProductAttributeRepository;
import com.vegusa.middleware.repository.ProductAttributeValueRepository;
import com.vegusa.middleware.utils.MWUtils;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.util.HashMap;
import java.util.Map;
import java.util.Objects;

@Service
public class ItemControlTableService {

    private final InterfaceDSRepository interfaces;
    private final InterfaceProductRepository interfaceProducts;
    private final ProductAttributeRepository attributes;
    private final ProductAttributeValueRepository attributeValues;

    @Autowired
    public ItemControlTableService(InterfaceDSRepository interfaces,
                                   InterfaceProductRepository interfaceProducts,
                                   ProductAttributeRepository attributes,
                                   ProductAttributeValueRepository attributeValues){
        this.interfaces = interfaces;
        this.interfaceProducts = interfaceProducts;
        this.attributes = attributes;
        this.attributeValues = attributeValues;
    }

    public String processAndSaveInterfaceInfo(String interfaceId, String dataAreaId) throws RuntimeException {
        JSONObject response = new JSONObject();
        InterfaceDS interfaceDS = interfaces.getInterface(interfaceId);
        if(!Objects.equals(interfaceDS.getCompany().getId().getDataAreaId(), dataAreaId)){
            throw new RuntimeException("The dataAreaId or interfaceId contains invalid information.");
        }
        InterfaceProduct[] interfaceProducts = this.interfaceProducts.getInterfaceProductsInfo(interfaceId, dataAreaId);
        for(InterfaceProduct iProduct : interfaceProducts){
            try {
                HashMap<String, String> uploadedInfo = new HashMap<>();
                Map<String, String> attributeValuesMap = getAttributeValuesMap(iProduct);
                saveInterfaceProduct(iProduct, attributeValuesMap);
                uploadedInfo.put("ok", iProduct.getItemId() + " uploaded successfully.");
                response.accumulate("uploaded", uploadedInfo);
                System.out.println(iProduct.getItemId() + " saved successfully.");
            } catch (RuntimeException e){
                HashMap<String, String> errorInfo = new HashMap<>();
                errorInfo.put("error", "Error when uploading the product " + iProduct.getItemId());
                response.accumulate("noUploaded", errorInfo);
                System.err.println("Error while uploading the product " + iProduct.getItemId());
            }
        }
        return response.toString();
    }

    private static Map<String, String> getAttributeValuesMap(InterfaceProduct iProduct) {
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
        return attributeValuesMap;
    }

    private void saveInterfaceProduct(InterfaceProduct interfaceProduct, Map<String, String> attributeValuesMap) throws RuntimeException {
        for (Map.Entry<String, String> attrValue : attributeValuesMap.entrySet()) {
            try {
                String itemId = interfaceProduct.getItemId(), interfaceId = interfaceProduct.getInterfaceField().getId().getInterfaceId(),
                        dataAreaId = interfaceProduct.getCompany().getId().getDataAreaId();
                ProductAttributeValue auxAttrValue = attributeValues.getProductAttributeValue(attrValue.getKey(), itemId, interfaceId, dataAreaId);
                if(attrValue.getValue() != null) {
                    ProductAttribute productAttribute = attributes.getProductAttribute(attrValue.getKey(), interfaceProduct.getCompany().getId().getDataAreaId());
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
                    attributeValues.save(attributeValue);
                } else if(auxAttrValue != null){
                    attributeValues.deleteProductAttributeValue(attrValue.getKey(), itemId, interfaceId, dataAreaId);
                }
            } catch (RuntimeException e) {
                System.err.println("Error while saving ProductAttributeValue to " + interfaceProduct.getItemId() + "-" + attrValue.getKey());
            }
        }
    }
}
