package com.vegusa.middleware.integrations.multivende.dto;

public class ProductAttributeDto {
    private ProductAttribute[] productAttribute;
    private ProductAttribute[] productVersionAttributes;
    private ProductCustomAttribute[] customAttributes;

    public ProductAttributeDto() {}

    public ProductAttributeDto(ProductAttribute[] productAttribute, ProductAttribute[] productVersionAttributes, ProductCustomAttribute[] customAttributes) {
        this.productAttribute = productAttribute;
        this.productVersionAttributes = productVersionAttributes;
        this.customAttributes = customAttributes;
    }

    public ProductAttribute[] getProductAttribute() {
        return productAttribute;
    }

    public void setProductAttribute(ProductAttribute[] productAttribute) {
        this.productAttribute = productAttribute;
    }

    public ProductAttribute[] getProductVersionAttributes() {
        return productVersionAttributes;
    }

    public void setProductVersionAttributes(ProductAttribute[] productVersionAttributes) {
        this.productVersionAttributes = productVersionAttributes;
    }

    public ProductCustomAttribute[] getCustomAttributes() {
        return customAttributes;
    }

    public void setCustomAttributes(ProductCustomAttribute[] customAttributes) {
        this.customAttributes = customAttributes;
    }
}
