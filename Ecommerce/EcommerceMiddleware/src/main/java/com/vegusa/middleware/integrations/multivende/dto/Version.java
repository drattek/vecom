package com.vegusa.middleware.integrations.multivende.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.math.BigDecimal;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class Version {
    @JsonProperty("_id")
    private String _id;
    private String code;
    private Integer position;
    private String productId;
    private Size size;
    private Color color;
    private BigDecimal weight;
    private BigDecimal length;
    private BigDecimal height;
    private BigDecimal width;
    @JsonProperty("InventoryTypeId")
    private String inventoryTypeId;

    public Version() {}

    public Version(String _id, String code, Integer position, String productId, Size size, Color color, BigDecimal weight, BigDecimal length, BigDecimal height, BigDecimal width, String inventoryTypeId) {
        this._id = _id;
        this.code = code;
        this.position = position;
        this.productId = productId;
        this.size = size;
        this.color = color;
        this.weight = weight;
        this.length = length;
        this.height = height;
        this.width = width;
        this.inventoryTypeId = inventoryTypeId;
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

    public String getInventoryTypeId() {
        return inventoryTypeId;
    }

    public void setInventoryTypeId(String inventoryTypeId) {
        this.inventoryTypeId = inventoryTypeId;
    }
}
