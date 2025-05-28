package com.vegusa.middleware.integrations.multivende.dto;

public class ProductAttribute {
    private String _id;
    private String name;
    private String code;
    private String description;
    private Integer position;

    public ProductAttribute() {}

    public ProductAttribute(String _id, String name, String code, String description, Integer position) {
        this._id = _id;
        this.name = name;
        this.code = code;
        this.description = description;
        this.position = position;
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

    public Integer getPosition() {
        return position;
    }

    public void setPosition(Integer position) {
        this.position = position;
    }
}
