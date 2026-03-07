package com.vegusa.middleware.integrations.mercadolibre.dto.product;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.vegusa.middleware.integrations.mercadolibre.dto.category.AttributeMeliDTO;

import java.math.BigDecimal;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class ProductMeliDTO {
    @JsonProperty("id")
    private String id;

    @JsonProperty("site_id")
    private String siteId;

    @JsonProperty("title")
    private String title;

    @JsonProperty("family_name")
    private String familyName;

    @JsonProperty("seller_id")
    private String sellerId;

    @JsonProperty("category_id")
    private String categoryId;

    @JsonProperty("user_product_id")
    private String userProductId;

    @JsonProperty("official_store_id")
    private String officialStoreId;

    @JsonProperty("price")
    private BigDecimal price;

    @JsonProperty("base_price")
    private BigDecimal basePrice;

    @JsonProperty("original_price")
    private BigDecimal originalPrice;

    @JsonProperty("currency_id")
    private String currencyId;

    @JsonProperty("initial_quantity")
    private Integer initialQuantity;

    @JsonProperty("available_quantity")
    private Integer availableQuantity;

    @JsonProperty("sold_quantity")
    private Integer soldQuantity;

    @JsonProperty("buying_mode")
    private String buyingMode;

    @JsonProperty("listing_type_id")
    private String listingTypeId;

    @JsonProperty("condition")
    private String condition;

    @JsonProperty("permalink")
    private String permalink;

    @JsonProperty("pictures")
    private PictureMeliDTO[] pictures;

    @JsonProperty("accepts_mercadopago")
    private Boolean acceptsMercadopago;

    @JsonProperty("tags")
    private String[] tags;

    @JsonProperty("status")
    private String status;

    @JsonProperty("sub_status")
    private String[] subStatus;

    @JsonProperty("domain_id")
    private String domainId;

    @JsonProperty("channels")
    private String[] channels;

    @JsonProperty("attributes")
    private AttributeMeliDTO[] attributes;

    @JsonProperty("shipping")
    private ShippingMeliDTO shipping;

    public ProductMeliDTO() {}

    public ProductMeliDTO(String id, String siteId, String title, String familyName, String sellerId, String categoryId, String userProductId, String officialStoreId, BigDecimal price, BigDecimal basePrice, BigDecimal originalPrice, String currencyId, Integer initialQuantity, Integer availableQuantity, Integer soldQuantity, String buyingMode, String listingTypeId, String condition, String permalink, PictureMeliDTO[] pictures, Boolean acceptsMercadopago, String[] tags, String status, String[] subStatus, String domainId, String[] channels, AttributeMeliDTO[] attributes, ShippingMeliDTO shipping) {
        this.id = id;
        this.siteId = siteId;
        this.title = title;
        this.familyName = familyName;
        this.sellerId = sellerId;
        this.categoryId = categoryId;
        this.userProductId = userProductId;
        this.officialStoreId = officialStoreId;
        this.price = price;
        this.basePrice = basePrice;
        this.originalPrice = originalPrice;
        this.currencyId = currencyId;
        this.initialQuantity = initialQuantity;
        this.availableQuantity = availableQuantity;
        this.soldQuantity = soldQuantity;
        this.buyingMode = buyingMode;
        this.listingTypeId = listingTypeId;
        this.condition = condition;
        this.permalink = permalink;
        this.pictures = pictures;
        this.acceptsMercadopago = acceptsMercadopago;
        this.tags = tags;
        this.status = status;
        this.subStatus = subStatus;
        this.domainId = domainId;
        this.channels = channels;
        this.attributes = attributes;
        this.shipping = shipping;
    }

    public String getId() {
        return id;
    }

    public void setId(String id) {
        this.id = id;
    }

    public String getSiteId() {
        return siteId;
    }

    public void setSiteId(String siteId) {
        this.siteId = siteId;
    }

    public String getTitle() {
        return title;
    }

    public void setTitle(String title) {
        this.title = title;
    }

    public String getFamilyName() {
        return familyName;
    }

    public void setFamilyName(String familyName) {
        this.familyName = familyName;
    }

    public String getSellerId() {
        return sellerId;
    }

    public void setSellerId(String sellerId) {
        this.sellerId = sellerId;
    }

    public String getCategoryId() {
        return categoryId;
    }

    public void setCategoryId(String categoryId) {
        this.categoryId = categoryId;
    }

    public String getUserProductId() {
        return userProductId;
    }

    public void setUserProductId(String userProductId) {
        this.userProductId = userProductId;
    }

    public String getOfficialStoreId() {
        return officialStoreId;
    }

    public void setOfficialStoreId(String officialStoreId) {
        this.officialStoreId = officialStoreId;
    }

    public BigDecimal getPrice() {
        return price;
    }

    public void setPrice(BigDecimal price) {
        this.price = price;
    }

    public BigDecimal getBasePrice() {
        return basePrice;
    }

    public void setBasePrice(BigDecimal basePrice) {
        this.basePrice = basePrice;
    }

    public BigDecimal getOriginalPrice() {
        return originalPrice;
    }

    public void setOriginalPrice(BigDecimal originalPrice) {
        this.originalPrice = originalPrice;
    }

    public String getCurrencyId() {
        return currencyId;
    }

    public void setCurrencyId(String currencyId) {
        this.currencyId = currencyId;
    }

    public Integer getInitialQuantity() {
        return initialQuantity;
    }

    public void setInitialQuantity(Integer initialQuantity) {
        this.initialQuantity = initialQuantity;
    }

    public Integer getAvailableQuantity() {
        return availableQuantity;
    }

    public void setAvailableQuantity(Integer availableQuantity) {
        this.availableQuantity = availableQuantity;
    }

    public Integer getSoldQuantity() {
        return soldQuantity;
    }

    public void setSoldQuantity(Integer soldQuantity) {
        this.soldQuantity = soldQuantity;
    }

    public String getBuyingMode() {
        return buyingMode;
    }

    public void setBuyingMode(String buyingMode) {
        this.buyingMode = buyingMode;
    }

    public String getListingTypeId() {
        return listingTypeId;
    }

    public void setListingTypeId(String listingTypeId) {
        this.listingTypeId = listingTypeId;
    }

    public String getCondition() {
        return condition;
    }

    public void setCondition(String condition) {
        this.condition = condition;
    }

    public String getPermalink() {
        return permalink;
    }

    public void setPermalink(String permalink) {
        this.permalink = permalink;
    }

    public PictureMeliDTO[] getPictures() {
        return pictures;
    }

    public void setPictures(PictureMeliDTO[] pictures) {
        this.pictures = pictures;
    }

    public Boolean getAcceptsMercadopago() {
        return acceptsMercadopago;
    }

    public void setAcceptsMercadopago(Boolean acceptsMercadopago) {
        this.acceptsMercadopago = acceptsMercadopago;
    }

    public String[] getTags() {
        return tags;
    }

    public void setTags(String[] tags) {
        this.tags = tags;
    }

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
    }

    public String[] getSubStatus() {
        return subStatus;
    }

    public void setSubStatus(String[] subStatus) {
        this.subStatus = subStatus;
    }

    public String getDomainId() {
        return domainId;
    }

    public void setDomainId(String domainId) {
        this.domainId = domainId;
    }

    public String[] getChannels() {
        return channels;
    }

    public void setChannels(String[] channels) {
        this.channels = channels;
    }

    public AttributeMeliDTO[] getAttributes() {
        return attributes;
    }

    public void setAttributes(AttributeMeliDTO[] attributes) {
        this.attributes = attributes;
    }

    public void setShipping(ShippingMeliDTO shipping) {
        this.shipping = shipping;
    }
}
