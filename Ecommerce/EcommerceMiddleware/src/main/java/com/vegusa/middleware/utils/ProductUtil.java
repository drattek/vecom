package com.vegusa.middleware.utils;

import com.vegusa.middleware.entity.InterfaceItems;
import com.vegusa.middleware.entity.ProductAttributeValue;
import com.vegusa.middleware.repository.local.*;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.BeanPropertyRowMapper;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Component;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.stream.Collectors;

@Component
public class ProductUtil {
    @Autowired
    private ProductAttributeValueRepository productAttributeValueRepository;

    @Autowired
    private ProductAttributeHierarchyRepository productAttributeHierarchyRepository;

    @Autowired
    private InterfaceHierarchyRepository interfaceHierarchyRepository;

    @Autowired
    private InterfaceItemsRepository interfaceItemsRepository;

    @Autowired
    private ProductAttributeRepository productAttributeRepository;

    @Autowired
    private JdbcTemplate jdbcTemplate;

    public InterfaceItems getAttributes(String internalCode, String dataAreaId){
        List<String> attributes = productAttributeRepository.getProductAttributeId(dataAreaId);
        Map<String, String> values = new HashMap<>();
        for (String attribute : attributes) {
            boolean skipNull = Boolean.parseBoolean(interfaceItemsRepository.getSkipNull(internalCode, dataAreaId));

            String responseAttribute = getAttributeValue(skipNull, attribute, internalCode, dataAreaId);
            values.put(attribute, responseAttribute);
        }

        return getBaseProduct(values);
    }

    public ProductAttributeValue getProductAttributesManual(String productAttributeId, String itemId, List<String> interfaceIds, String dataAreaId) {
        String inSql = interfaceIds.stream()
                .map(id -> "'" + id + "'")
                .collect(Collectors.joining(","));

        String sql = String.format(
                "SELECT * FROM productattributevalue WHERE ProductAttributeId = ? AND ItemId = ? AND InterfaceId IN (%s) AND DataAreaId = ? AND Value IS NOT NULL ORDER BY FIELD(InterfaceId, %s) LIMIT 1",
                inSql, inSql
        );

        List<ProductAttributeValue> results = jdbcTemplate.query(
                sql,
                new Object[] { productAttributeId, itemId, dataAreaId },
                new BeanPropertyRowMapper<>(ProductAttributeValue.class)
        );

        if (results.isEmpty()) {
            return null;
        } else {
            return results.getFirst();
        }
    }

    private List<String> getHierarchies(String attribute, String dataAreaId){
        List<String> hierarchies = productAttributeHierarchyRepository.getAttributeHierarchy(attribute, dataAreaId);
        if (hierarchies.isEmpty()) {
            hierarchies = interfaceHierarchyRepository.getInterfaceHierarchy(dataAreaId);
        }

        return hierarchies;
    }

    private String getAttributeValue(boolean skipNull, String attribute, String internalCode, String dataAreaId){
        List<String> hierarchies = getHierarchies(attribute, dataAreaId);
        String responseAttribute = "";
        if (skipNull){
            String priorityList = hierarchies.stream()
                    .map(s -> "'" + s + "'")
                    .collect(Collectors.joining(","));
            /*
            ProductAttributeValue value = productAttributeValueRepository.getProductAttributes(
                    attribute,
                    internalCode,
                    hierarchies,
                    dataAreaId,
                    priorityList
            );
            */
            ProductAttributeValue value = getProductAttributesManual(attribute, internalCode, hierarchies, dataAreaId);
            if (value != null) responseAttribute = value.getValue();
        } else {
            ProductAttributeValue value = productAttributeValueRepository.getProductAttributeValue(
                    attribute,
                    internalCode,
                    hierarchies.getFirst(),
                    dataAreaId
            );
            if (value != null) responseAttribute = value.getValue();
        }

        return responseAttribute;
    }

    private InterfaceItems getBaseProduct(Map<String, String> attributes){
        InterfaceItems product = new InterfaceItems();
        product.setItemId(attributes.get("ITEM_ID"));
        product.setProductName(attributes.get("PRODUCT_NAME"));
        product.setPartNumber(attributes.get("PART_NUMBER"));
        product.setShortDescription(attributes.get("SHORT_DESCRIPTION"));
        product.setBrand(attributes.get("BRAND_ID"));
        product.setCategory(attributes.get("CATEGORY_ID"));
        product.setWeight(attributes.get("WEIGHT"));
        product.setUnitOfMeasurement(attributes.get("UNIT_OF_MEASUREMENT"));
        if (attributes.getOrDefault("AVAILABLE", null) != null){
            product.setAvailable(Double.parseDouble(attributes.get("AVAILABLE")));
        }
        if (attributes.getOrDefault("COST", null) != null){
            product.setCost(Double.parseDouble(attributes.get("COST")));
        }
        product.setLength(attributes.get("LENGTH"));
        product.setHeight(attributes.get("HEIGHT"));
        product.setWeight(attributes.get("WIDTH"));
        product.setSeoTitle(attributes.get("SEO_TITLE"));
        product.setMetaDescription(attributes.get("META_DESCRIPTION"));
        product.setCrossReferences(attributes.get("CROSS_REFERENCES"));

        return product;
    }

    public String getShortDescription(String shortDescription, String productName) {
        StringBuilder description = new StringBuilder();

        if (shortDescription != null && !shortDescription.isBlank()) {
            description.append(shortDescription);
        }

        if (productName != null && !productName.isBlank()) {
            if (!description.isEmpty()) {
                description.append(" - ");
            }
            description.append(productName);
        }

        return description.toString();
    }

    public String getName(InterfaceItems product, String shortDescription) {
        StringBuilder nameBuilder = new StringBuilder();

        if (shortDescription != null && !shortDescription.isBlank()) {
            nameBuilder.append(shortDescription);
        }

        String brand = product.getBrand();
        if (brand != null && !brand.isBlank() && !nameBuilder.toString().contains(brand)) {
            if (!nameBuilder.isEmpty()) nameBuilder.append(" ");
            nameBuilder.append(brand);
        }

        String partNumber = product.getPartNumber();
        if (partNumber != null && !partNumber.isBlank() && !nameBuilder.toString().contains(partNumber)) {
            if (!nameBuilder.isEmpty()) nameBuilder.append(" ");
            nameBuilder.append(partNumber);
        }

        return nameBuilder.toString();
    }

    public String getCrossReferences(String crossReferences) {
        String response = "";
        if(!Objects.equals(crossReferences, "")){
            response = "Equivalente con: " + crossReferences;
        }
        return response;
    }
}
