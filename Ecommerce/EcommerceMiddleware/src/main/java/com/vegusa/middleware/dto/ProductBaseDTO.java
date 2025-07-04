package com.vegusa.middleware.dto;

import com.fasterxml.jackson.annotation.JsonInclude;

import java.math.BigDecimal;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class ProductBaseDTO {
    private String internalCode;
    private String name;
    private String partNumber;
    private String description;
    private String brand;
    private String category;
    private String unitOfMeasurement;
    private String available;
    private BigDecimal cost;
    private BigDecimal weight;
    private BigDecimal length;
    private BigDecimal height;
    private BigDecimal width;
    private String warranty;
    private String crossReferences;

    public ProductBaseDTO() {}

    public ProductBaseDTO(String internalCode, String name, String partNumber, String description, String brand, String category, String unitOfMeasurement, String available, BigDecimal cost, BigDecimal weight, BigDecimal length, BigDecimal height, BigDecimal width, String warranty, String crossReferences) {
        this.internalCode = internalCode;
        this.name = name;
        this.partNumber = partNumber;
        this.description = description;
        this.brand = brand;
        this.category = category;
        this.unitOfMeasurement = unitOfMeasurement;
        this.available = available;
        this.cost = cost;
        this.weight = weight;
        this.length = length;
        this.height = height;
        this.width = width;
        this.warranty = warranty;
        this.crossReferences = crossReferences;
    }

    public String getInternalCode() {
        return internalCode;
    }

    public void setInternalCode(String internalCode) {
        this.internalCode = internalCode;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getPartNumber() {
        return partNumber;
    }

    public void setPartNumber(String partNumber) {
        this.partNumber = partNumber;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getBrand() {
        return brand;
    }

    public void setBrand(String brand) {
        this.brand = brand;
    }

    public String getCategory() {
        return category;
    }

    public void setCategory(String category) {
        this.category = category;
    }

    public String getUnitOfMeasurement() {
        return unitOfMeasurement;
    }

    public void setUnitOfMeasurement(String unitOfMeasurement) {
        this.unitOfMeasurement = unitOfMeasurement;
    }

    public String getAvailable() {
        return available;
    }

    public void setAvailable(String available) {
        this.available = available;
    }

    public BigDecimal getCost() {
        return cost;
    }

    public void setCost(BigDecimal cost) {
        this.cost = cost;
    }

    public BigDecimal getWeight() {
        return weight;
    }

    public void setWeight(BigDecimal weight) {
        this.weight = weight;
    }

    public BigDecimal getLength() {
        return length;
    }

    public void setLength(BigDecimal length) {
        this.length = length;
    }

    public BigDecimal getHeight() {
        return height;
    }

    public void setHeight(BigDecimal height) {
        this.height = height;
    }

    public BigDecimal getWidth() {
        return width;
    }

    public void setWidth(BigDecimal width) {
        this.width = width;
    }

    public String getWarranty() {
        return warranty;
    }

    public void setWarranty(String warranty) {
        this.warranty = warranty;
    }

    public String getCrossReferences() {
        return crossReferences;
    }

    public void setCrossReferences(String crossReferences) {
        this.crossReferences = crossReferences;
    }
}
