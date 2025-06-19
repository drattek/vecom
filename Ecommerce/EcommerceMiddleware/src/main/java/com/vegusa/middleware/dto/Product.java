package com.vegusa.middleware.dto;

public class Product {
    private Long id;
    private String responseId;
    private String name;
    private String alias;
    private String model;
    private String description;
    private String brandId;
    private String code;
    private String internalCode;
    private String shortDescription;

    public Product() {}

    public Product(Long id, String responseId, String name, String alias, String model, String description, String brandId, String code, String internalCode, String shortDescription) {
        this.id = id;
        this.responseId = responseId;
        this.name = name;
        this.alias = alias;
        this.model = model;
        this.description = description;
        this.brandId = brandId;
        this.code = code;
        this.internalCode = internalCode;
        this.shortDescription = shortDescription;
    }

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getResponseId() {
        return responseId;
    }

    public void setResponseId(String responseId) {
        this.responseId = responseId;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getAlias() {
        return alias;
    }

    public void setAlias(String alias) {
        this.alias = alias;
    }

    public String getModel() {
        return model;
    }

    public void setModel(String model) {
        this.model = model;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getBrandId() {
        return brandId;
    }

    public void setBrandId(String brandId) {
        this.brandId = brandId;
    }

    public String getCode() {
        return code;
    }

    public void setCode(String code) {
        this.code = code;
    }

    public String getInternalCode() {
        return internalCode;
    }

    public void setInternalCode(String internalCode) {
        this.internalCode = internalCode;
    }

    public String getShortDescription() {
        return shortDescription;
    }

    public void setShortDescription(String shortDescription) {
        this.shortDescription = shortDescription;
    }
}
