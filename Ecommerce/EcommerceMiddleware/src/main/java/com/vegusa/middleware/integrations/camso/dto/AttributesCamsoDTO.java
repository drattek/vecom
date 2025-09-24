package com.vegusa.middleware.integrations.camso.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.math.BigDecimal;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class AttributesCamsoDTO {
    @JsonProperty("name")
    private String name;

    @JsonProperty("partNumber")
    private String partNumber;

    @JsonProperty("diameter")
    private BigDecimal diameter;

    @JsonProperty("width")
    private BigDecimal width;

    @JsonProperty("height")
    private BigDecimal height;

    @JsonProperty("length")
    private BigDecimal length;

    @JsonProperty("weight")
    private BigDecimal weight;

    @JsonProperty("code")
    private String barcode;

    public AttributesCamsoDTO() {}

    public AttributesCamsoDTO(String name, String partNumber, BigDecimal diameter, BigDecimal width, BigDecimal height, BigDecimal length, BigDecimal weight, String barcode) {
        this.name = name;
        this.partNumber = partNumber;
        this.diameter = diameter;
        this.width = width;
        this.height = height;
        this.length = length;
        this.weight = weight;
        this.barcode = barcode;
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

    public BigDecimal getDiameter() {
        return diameter;
    }

    public void setDiameter(BigDecimal diameter) {
        this.diameter = diameter;
    }

    public BigDecimal getWidth() {
        return width;
    }

    public void setWidth(BigDecimal width) {
        this.width = width;
    }

    public BigDecimal getHeight() {
        return height;
    }

    public void setHeight(BigDecimal height) {
        this.height = height;
    }

    public BigDecimal getLength() {
        return length;
    }

    public void setLength(BigDecimal length) {
        this.length = length;
    }

    public BigDecimal getWeight() {
        return weight;
    }

    public void setWeight(BigDecimal weight) {
        this.weight = weight;
    }

    public String getBarcode() {
        return barcode;
    }

    public void setBarcode(String barcode) {
        this.barcode = barcode;
    }
}
