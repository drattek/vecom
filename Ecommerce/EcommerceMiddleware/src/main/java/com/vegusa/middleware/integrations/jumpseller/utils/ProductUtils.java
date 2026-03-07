package com.vegusa.middleware.integrations.jumpseller.utils;

import com.vegusa.middleware.entity.ProductAttributeValue;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerProductDto;
import com.vegusa.middleware.integrations.jumpseller.dto.Product;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncProductJumpsellerRepository;
import com.vegusa.middleware.repository.local.*;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import java.util.*;
import java.util.stream.Collectors;

@Component
public class ProductUtils {

    @Autowired
    private ProductAttributeRepository productAttributeRepository;

    @Autowired
    private ProductAttributeHierarchyRepository productAttributeHierarchyRepository;

    @Autowired
    private InterfaceHierarchyRepository interfaceHierarchyRepository;

    @Autowired
    private InterfaceItemsRepository interfaceItemsRepository;

    @Autowired
    private ProductAttributeValueRepository productAttributeValueRepository;

    @Autowired
    private SyncProductJumpsellerRepository syncProductJumpsellerRepository;

    public SyncJumpsellerProduct toEntity(JumpsellerProductDto dto, String internalCode){
        SyncJumpsellerProduct entity = syncProductJumpsellerRepository.findByResponseId(dto.getProduct().getId())
                .orElseGet(SyncJumpsellerProduct::new);
        Product product = dto.getProduct();
        entity.setResponseId(product.getId());
        entity.setInternalCode(internalCode);
        entity.setName(product.getName());
        entity.setPageTitle(product.getPage_title());
        entity.setDescription(product.getDescription());
        entity.setMetaDescription(product.getMeta_description());
        entity.setType(product.getType());
        entity.setDaysToExpire(product.getDays_to_expire());
        entity.setPrice(product.getPrice());
        entity.setDiscount(product.getDiscount());
        entity.setWeight(product.getWeight());
        entity.setStock(product.getStock());
        entity.setStockUnlimited(product.isStock_unlimited());
        entity.setStockThreshold(product.getStock_threshold());
        entity.setStockNotification(product.isStock_notification());
        entity.setCostPerItem(product.getCost_per_item());
        entity.setCompareAtPrice(product.getCompare_at_price());
        if (product.getMinimum_quantity() != null) entity.setMinimumQuantity(dto.getProduct().getMinimum_quantity());
        if (product.getMaximum_quantity() != null) entity.setMaximumQuantity(dto.getProduct().getMaximum_quantity());
        entity.setSku(dto.getProduct().getSku());
        entity.setBrand(dto.getProduct().getBrand());
        entity.setBarcode(dto.getProduct().getBarcode());
        entity.setGoogleProductCategory(dto.getProduct().getGoogle_product_category());
        entity.setFeatured(dto.getProduct().isFeatured());
        entity.setShippingRequired(dto.getProduct().isShipping_required());
        entity.setReviewsEnabled(dto.getProduct().isReviews_enabled());
        entity.setStatus(dto.getProduct().getStatus());
        entity.setCreatedAt(dto.getProduct().getCreated_at());
        entity.setUpdatedAt(dto.getProduct().getUpdated_at());
        entity.setPackageFormat(dto.getProduct().getPackage_format());
        entity.setLength(dto.getProduct().getLength());
        entity.setWidth(dto.getProduct().getWidth());
        entity.setHeight(dto.getProduct().getHeight());
        entity.setDiameter(dto.getProduct().getDiameter());
        entity.setPermalink(dto.getProduct().getPermalink());
        entity.setDataAreaId("MSB");
        entity.setCompanyRefRecId(1L);

        return entity;
    }

    public JumpsellerProductDto toDto(SyncJumpsellerProduct entity){
        Product product = new Product();
        JumpsellerProductDto dto = new JumpsellerProductDto(product);

        dto.getProduct().setName(entity.getName());
        dto.getProduct().setDescription(entity.getDescription());
        dto.getProduct().setPage_title(entity.getPageTitle());
        dto.getProduct().setMeta_description(entity.getMetaDescription());
        dto.getProduct().setType(entity.getType());
        dto.getProduct().setDays_to_expire(entity.getDaysToExpire());
        dto.getProduct().setPrice(entity.getPrice());
        dto.getProduct().setWeight(entity.getWeight());
        dto.getProduct().setStock(entity.getStock());
        dto.getProduct().setStock_unlimited(entity.isStockUnlimited());
        dto.getProduct().setStock_threshold(entity.getStockThreshold());
        dto.getProduct().setStock_notification(entity.isStockNotification());
        dto.getProduct().setCost_per_item(entity.getCostPerItem());
        dto.getProduct().setCompare_at_price(entity.getCompareAtPrice());
        dto.getProduct().setMinimum_quantity(entity.getMinimumQuantity());
        dto.getProduct().setMaximum_quantity(entity.getMaximumQuantity());
        dto.getProduct().setSku(entity.getSku());
        dto.getProduct().setBarcode(entity.getBarcode());
        dto.getProduct().setGoogle_product_category(entity.getGoogleProductCategory());
        dto.getProduct().setFeatured(entity.isFeatured());
        dto.getProduct().setShipping_required(entity.isShippingRequired());
        dto.getProduct().setStatus("available");
        dto.getProduct().setPackage_format(entity.getPackageFormat());
        dto.getProduct().setLength(entity.getLength());
        dto.getProduct().setWidth(entity.getWidth());
        dto.getProduct().setHeight(entity.getHeight());
        dto.getProduct().setDiameter(entity.getDiameter());
        dto.getProduct().setPermalink(entity.getPermalink());

        return dto;
    }

    public HashMap<String, String> getPrices(JSONArray itemValues) {
        HashMap<String, String> response = new HashMap<>();

        for (int i = 0; i < itemValues.length(); i++){
            try {
                JSONObject obj = itemValues.getJSONObject(i);
                String code = obj.getString("code");
                String price = obj.getString("price");
                if (code != null && price != null) {
                    response.put(code, price);
                }
            } catch (RuntimeException e){
                System.err.println("An error occurred transforming the price list.");
            }
        }
        return response;
    }

    public String generateName(HashMap<String, String> data){
        String name = data.get("SHORT_DESCRIPTION");
        boolean hasBrand = name.contains(data.get("BRAND_ID"));
        boolean hasPartNumber = name.contains(data.get("PART_NUMBER"));

        if (!hasBrand && !Objects.equals(data.get("BRAND_ID"), "")) {
            name = !Objects.equals(name, "") ? name + " " + data.get("BRAND_ID") : data.get("BRAND_ID");
        }
        if (!hasPartNumber && !Objects.equals(data.get("PART_NUMBER"), "")) {
            name = !Objects.equals(name, "") ? name + " " + data.get("PART_NUMBER") : data.get("PART_NUMBER");
        }
        return name;
    }

    public HashMap<String, String> getAttributes(SyncJumpsellerProduct entity, String dataAreaId){
        List<String> attributes = productAttributeRepository.getProductAttributeId(dataAreaId);
        HashMap<String, String> values = new HashMap<>();
        for (String attribute : attributes) {
            List<String> hierarchies = productAttributeHierarchyRepository.getAttributeHierarchy(attribute, dataAreaId);
            if (hierarchies.isEmpty()) {
                hierarchies = interfaceHierarchyRepository.getInterfaceHierarchy(dataAreaId);
            }
            boolean skipNull = Boolean.parseBoolean(interfaceItemsRepository.getSkipNull(entity.getInternalCode(), dataAreaId));

            String responseAttribute = "";
            if (skipNull){
                String priorityList = hierarchies.stream()
                        .map(s -> "'" + s + "'")
                        .collect(Collectors.joining(","));
                ProductAttributeValue value = productAttributeValueRepository.getProductAttributes(
                        attribute,
                        entity.getInternalCode(),
                        hierarchies,
                        dataAreaId,
                        priorityList
                );
                if (value != null) responseAttribute = value.getValue();
            } else {
                ProductAttributeValue value = productAttributeValueRepository.getProductAttributeValue(
                        attribute,
                        entity.getInternalCode(),
                        hierarchies.get(0),
                        dataAreaId
                );
                if (value != null) responseAttribute = value.getValue();
            }
            values.put(attribute, responseAttribute);
        }

        return values;
    }

    public JumpsellerProductDto getProduct(HashMap<String, String> values, SyncJumpsellerProduct entity){
        Product product = new Product();
        String name = generateName(values);
        String shortDescription = values.get("PRODUCT_NAME") + " - " + name;

        product.setSku(values.get("PART_NUMBER"));
        product.setName(name);
        product.setBrand(values.get("BRAND_ID"));
        product.setPrice(entity.getPrice());
        String description = "-- TIENDA VEGUSA MAQUINARIA, DISTRUIBIDOR AUTORIZADO UNICARRIERS, BOBCAT, JLG, FLEXI. -- " + (!values.get("META_DESCRIPTION").isBlank() ? values.get("META_DESCRIPTION") : shortDescription) + ". " + values.get("CROSS_REFERENCES");
        product.setDescription(description);
        product.setPage_title(!values.get("SEO_TITLE").isBlank() ? values.get("SEO_TITLE") : name);
        product.setMeta_description(!values.get("META_DESCRIPTION").isBlank() ? values.get("META_DESCRIPTION") : shortDescription);
//        double weight = Double.parseDouble(!values.get("WEIGHT").isBlank() ? values.get("WEIGHT") : "0.0");
//        product.setWeight(Math.max(weight, 1.0));
//        product.setStatus("available");
//        product.setLength(Double.parseDouble(!values.get("LENGTH").isBlank() ? values.get("LENGTH") : "0.0"));
//        product.setWidth(Double.parseDouble(!values.get("WIDTH").isBlank() ? values.get("WIDTH") : "0.0"));
//        product.setHeight(Double.parseDouble(!values.get("HEIGHT").isBlank() ? values.get("HEIGHT") : "0.0"));

        return new JumpsellerProductDto(product);
    }

    public List<List<String>> getChunks(SyncJumpsellerProduct[] products, int size){
        List<List<String>> chunks = new ArrayList<>();
        List<SyncJumpsellerProduct> products_array = Arrays.asList(products);

        for (int i = 0; i < products_array.size(); i += size) {
            List<String> chunk = products_array.subList(i, Math.min(i + size, products_array.size()))
                    .stream()
                    .map(product -> product.getResponseId().toString())
                    .collect(Collectors.toList());
            chunks.add(chunk);
        }

        return chunks;
    }
}
