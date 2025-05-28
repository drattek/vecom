package com.vegusa.middleware.integrations.multivende.dto;

public class Product {
    private String _id;
    private String name;
    private String code;
    private String description;
    private String htmlDescription;
    private String shortDescription;
    private String model;
    private String createdAt;
    private String updatedAt;
    private String status;
    private String brandId;
    private String seasonId;
    private String codeTypeId;
    private String productTypeId;
    private String productCategoryId;
    private String internalCodeTypeId;
    private Brand brand;
    private Season season;
    private Category productCategory;
    private Tag[] productTags;
    private Version[] productVersions;

    public static class Version {
        private String _id;
        private String code;
        private Integer position;
        private String productId;
        private Size size;
        private Color color;

        public Version() {}

        public Version(String _id, String code, Integer position, String productId, Size size, Color color) {
            this._id = _id;
            this.code = code;
            this.position = position;
            this.productId = productId;
            this.size = size;
            this.color = color;
        }

        public String get_id() {
            return _id;
        }

        public void set_id(String _id) {
            this._id = _id;
        }

        public String getCode() {
            return code;
        }

        public void setCode(String code) {
            this.code = code;
        }

        public Integer getPosition() {
            return position;
        }

        public void setPosition(Integer position) {
            this.position = position;
        }

        public String getProductId() {
            return productId;
        }

        public void setProductId(String productId) {
            this.productId = productId;
        }

        public Size getSize() {
            return size;
        }

        public void setSize(Size size) {
            this.size = size;
        }

        public Color getColor() {
            return color;
        }

        public void setColor(Color color) {
            this.color = color;
        }
    }

    public Product() {}

    public Product(String _id, String name, String code, String description, String htmlDescription, String shortDescription, String model, String createdAt, String updatedAt, String status, String brandId, String seasonId, String codeTypeId, String productTypeId, String productCategoryId, String internalCodeTypeId, Brand brand, Season season, Category productCategory, Tag[] productTags, Version[] productVersions) {
        this._id = _id;
        this.name = name;
        this.code = code;
        this.description = description;
        this.htmlDescription = htmlDescription;
        this.shortDescription = shortDescription;
        this.model = model;
        this.createdAt = createdAt;
        this.updatedAt = updatedAt;
        this.status = status;
        this.brandId = brandId;
        this.seasonId = seasonId;
        this.codeTypeId = codeTypeId;
        this.productTypeId = productTypeId;
        this.productCategoryId = productCategoryId;
        this.internalCodeTypeId = internalCodeTypeId;
        this.brand = brand;
        this.season = season;
        this.productCategory = productCategory;
        this.productTags = productTags;
        this.productVersions = productVersions;
    }

    public String get_id() {
        return _id;
    }

    public void set_id(String _id) {
        this._id = _id;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getCode() {
        return code;
    }

    public void setCode(String code) {
        this.code = code;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getHtmlDescription() {
        return htmlDescription;
    }

    public void setHtmlDescription(String htmlDescription) {
        this.htmlDescription = htmlDescription;
    }

    public String getShortDescription() {
        return shortDescription;
    }

    public void setShortDescription(String shortDescription) {
        this.shortDescription = shortDescription;
    }

    public String getModel() {
        return model;
    }

    public void setModel(String model) {
        this.model = model;
    }

    public String getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(String createdAt) {
        this.createdAt = createdAt;
    }

    public String getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(String updatedAt) {
        this.updatedAt = updatedAt;
    }

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
    }

    public String getBrandId() {
        return brandId;
    }

    public void setBrandId(String brandId) {
        this.brandId = brandId;
    }

    public String getSeasonId() {
        return seasonId;
    }

    public void setSeasonId(String seasonId) {
        this.seasonId = seasonId;
    }

    public String getCodeTypeId() {
        return codeTypeId;
    }

    public void setCodeTypeId(String codeTypeId) {
        this.codeTypeId = codeTypeId;
    }

    public String getProductTypeId() {
        return productTypeId;
    }

    public void setProductTypeId(String productTypeId) {
        this.productTypeId = productTypeId;
    }

    public String getProductCategoryId() {
        return productCategoryId;
    }

    public void setProductCategoryId(String productCategoryId) {
        this.productCategoryId = productCategoryId;
    }

    public String getInternalCodeTypeId() {
        return internalCodeTypeId;
    }

    public void setInternalCodeTypeId(String internalCodeTypeId) {
        this.internalCodeTypeId = internalCodeTypeId;
    }

    public Brand getBrand() {
        return brand;
    }

    public void setBrand(Brand brand) {
        this.brand = brand;
    }

    public Season getSeason() {
        return season;
    }

    public void setSeason(Season season) {
        this.season = season;
    }

    public Category getProductCategory() {
        return productCategory;
    }

    public void setProductCategory(Category productCategory) {
        this.productCategory = productCategory;
    }

    public Tag[] getProductTags() {
        return productTags;
    }

    public void setProductTags(Tag[] productTags) {
        this.productTags = productTags;
    }

    public Version[] getProductVersions() {
        return productVersions;
    }

    public void setProductVersions(Version[] productVersions) {
        this.productVersions = productVersions;
    }
}
