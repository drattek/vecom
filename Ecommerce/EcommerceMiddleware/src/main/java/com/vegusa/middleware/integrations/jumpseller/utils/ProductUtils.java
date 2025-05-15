package com.vegusa.middleware.integrations.jumpseller.utils;

import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerProductDto;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import org.springframework.stereotype.Component;

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
}
