package com.vegusa.veg_mv_integration_midd.msb.service;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.InterfaceDS;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.InterfaceProduct;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.ProductAttribute;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.ProductAttributeValue;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.InterfaceDSRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.InterfaceProductRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.ProductAttributeRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.ProductAttributeValueRepository;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.util.HashMap;
import java.util.Map;
import java.util.Objects;

@Service
public class ProductControlTableService{

    private final InterfaceDSRepository interfaces;
    private final InterfaceProductRepository interfaceProducts;
    private final ProductAttributeRepository attributes;
    private final ProductAttributeValueRepository attributeValues;

    @Autowired
    public ProductControlTableService(InterfaceDSRepository interfaces,
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
        Map<String, String> attributeValuesMap = new HashMap<String, String>();
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
                ProductAttributeValue auxAttrValue = attributeValues.getProductAttributeValue(attrValue.getKey(), interfaceProduct.getItemId(), interfaceProduct.getInterfaceField().getId().getInterfaceId(), interfaceProduct.getCompany().getId().getDataAreaId());
                if(attrValue.getValue() != null || auxAttrValue != null) {
                    ProductAttribute productAttribute = attributes.getProductAttribute(attrValue.getKey());
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
                }
            } catch (RuntimeException e) {
                System.err.println("Error while saving ProductAttributeValue to " + interfaceProduct.getItemId() + "-" + attrValue.getKey());
            }
        }
    }
}
