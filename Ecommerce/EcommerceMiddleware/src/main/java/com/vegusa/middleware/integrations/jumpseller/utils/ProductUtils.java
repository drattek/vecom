package com.vegusa.middleware.integrations.jumpseller.utils;

import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerProductDto;
import com.vegusa.middleware.integrations.jumpseller.dto.Product;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.stereotype.Component;

import java.util.HashMap;

@Component
public class ProductUtils {

    public SyncJumpsellerProduct toEntity(JumpsellerProductDto dto, String internalCode){
        SyncJumpsellerProduct entity = new SyncJumpsellerProduct();
        entity.setResponseId(dto.getProduct().getId());
        entity.setInternalCode(internalCode);
        entity.setName(dto.getProduct().getName());
        entity.setPageTitle(dto.getProduct().getPage_title());
        entity.setDescription(dto.getProduct().getDescription());
        entity.setMetaDescription(dto.getProduct().getMeta_description());
        entity.setType(dto.getProduct().getType());
        entity.setDaysToExpire(dto.getProduct().getDays_to_expire());
        entity.setPrice(dto.getProduct().getPrice());
        entity.setDiscount(dto.getProduct().getDiscount());
        entity.setWeight(dto.getProduct().getWeight());
        entity.setStock(dto.getProduct().getStock());
        entity.setStockUnlimited(dto.getProduct().isStock_unlimited());
        entity.setStockThreshold(dto.getProduct().getStock_threshold());
        entity.setStockNotification(dto.getProduct().isStock_notification());
        entity.setCostPerItem(dto.getProduct().getCost_per_item());
        entity.setCompareAtPrice(dto.getProduct().getCompare_at_price());
        entity.setMinimumQuantity(dto.getProduct().getMinimum_quantity());
        entity.setMaximumQuantity(dto.getProduct().getMaximum_quantity());
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
}
