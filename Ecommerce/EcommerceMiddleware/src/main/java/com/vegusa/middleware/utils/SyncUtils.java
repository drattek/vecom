package com.vegusa.middleware.utils;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.entity.ProductAttributeValue;
import com.vegusa.middleware.entity.Products;
import com.vegusa.middleware.repository.*;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.HashMap;
import java.util.List;
import java.util.Objects;

@Component
public class SyncUtils {
    @Autowired
    private ProductAttributeRepository attributeRepository;

    @Autowired
    private ProductAttributeValueRepository attributeValueRepository;

    @Autowired
    private ProductAttributeHierarchyRepository hierarchyRepository;

    @Autowired
    private InterfaceHierarchyRepository interfaceHierarchyRepository;

    @Autowired
    private ProductsRepository productsRepository;

    public String getName(Products product, String shortDescription) {
        String name = shortDescription;
        boolean hasBrand = name.contains(product.getBrand());
        boolean hasPartNumber = name.contains(product.getPartNumber());
        if(!hasBrand && !Objects.equals(product.getBrand(), "")){
            name = !Objects.equals(name, "") ? name + " "  + product.getBrand() : product.getBrand();
        }
        if(!hasPartNumber && !Objects.equals(product.getPartNumber(), "")){
            name = !Objects.equals(name, "") ? name + " " + product.getPartNumber() : product.getPartNumber();
        }
        return capitalize(name);
    }

    public String getDescription(Products product, String name){
        if (!product.getMetaDescription().isBlank()) {
            return product.getMetaDescription();
        }

        String prefix = "-- Tienda Vegusa Maquinaria, Distribuidor Autorizado Unicarriers, Bobcat, JLG, Flexi. -- ";

        return prefix + name + " - " + product.getItemId();
    }

    private String capitalize(String str) {
        if (str == null || str.isEmpty()) {
            return str;
        }
        return str.substring(0, 1).toUpperCase() + str.substring(1).toLowerCase();
    }

    public Products getProductValues(String internalCode){
        List<String> attributes = attributeRepository.getProductAttributeId(DataArea.MSB.name());
        HashMap<String, String> attributeMap = new HashMap<>();
        List<String> auxHierarchy;
        String auxAttributeValue;
        for (String attribute : attributes){
            auxHierarchy = hierarchyRepository.getAttributeHierarchy(attribute, DataArea.MSB.name());
            if (auxHierarchy.isEmpty()){
                auxHierarchy = interfaceHierarchyRepository.getInterfaceHierarchy(DataArea.MSB.name());
            }

            auxAttributeValue = getAttributeValue(auxHierarchy, attribute, internalCode);
            attributeMap.put(attribute, auxAttributeValue);
        }

        return getProductBase(attributeMap);
    }

    private String getAttributeValue(List<String> hierarchies, String attribute, String itemId){
        boolean skipNull = Boolean.parseBoolean(productsRepository.getSkipNull(itemId, DataArea.MSB.name()));
        String response = "";

        for (String hierarchy : hierarchies) {
            ProductAttributeValue auxAttributeValue = attributeValueRepository.getProductAttributeValue(attribute, itemId, hierarchy, DataArea.MSB.name());
            if (auxAttributeValue != null) {
                response = auxAttributeValue.getValue();
                break;
            }
            if (!skipNull) {
                break;
            }
        }

        return response;
    }

    private Products getProductBase(HashMap<String, String> values) {
        Products product = new Products();
        product.setItemId(values.get("ITEM_ID"));
        product.setProductName(values.get("PRODUCT_NAME"));
        product.setPartNumber(values.get("PART_NUMBER"));
        product.setShortDescription(values.get("SHORT_DESCRIPTION"));
        product.setBrand(values.get("BRAND_ID"));
        product.setCategory(values.get("CATEGORY_ID"));
        BigDecimal weight = values.get("WEIGHT") != null && !values.get("WEIGHT").isBlank() ? new BigDecimal(values.get("WEIGHT")) : BigDecimal.ZERO;
        product.setWeight(weight);
        product.setUnitOfMeasurement(values.get("UNIT_OF_MEASUREMENT"));
        if(values.get("AVAILABLE") != null && !values.get("AVAILABLE").isBlank()){
            product.setAvailable(new BigDecimal(values.get("AVAILABLE")).longValue());
        }
        if(values.get("COST") != null || !values.get("COST").isBlank()){
            product.setCost(new BigDecimal(values.getOrDefault("COST", "0.00")));
        }
        BigDecimal length = values.get("LENGTH") != null && !values.get("LENGTH").isBlank() ? new BigDecimal(values.get("LENGTH")) : BigDecimal.ZERO;
        product.setLength(length);
        BigDecimal height = values.get("HEIGHT") != null && !values.get("HEIGHT").isBlank() ? new BigDecimal(values.get("HEIGHT")) : BigDecimal.ZERO;
        product.setHeight(height);
        BigDecimal width = values.get("WIDTH") != null && !values.get("WIDTH").isBlank() ? new BigDecimal(values.get("WIDTH")) : BigDecimal.ZERO;
        product.setWidth(width);
        product.setSeoTitle(values.get("SEO_TITLE"));
        product.setMetaDescription(values.get("META_DESCRIPTION"));
        product.setCrossReferences(values.get("CROSS_REFERENCES"));
        product.setUpdatedAt(Instant.now());

        return product;
    }
}
