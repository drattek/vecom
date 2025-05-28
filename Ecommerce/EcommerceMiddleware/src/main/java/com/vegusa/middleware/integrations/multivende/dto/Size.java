package com.vegusa.middleware.integrations.multivende.dto;

public class Size {
    private String _id;
    private Integer position;
    private String name;
    private String description;
    private String merchantId;

    public Size() {}

    public Size(String _id, Integer position, String name, String description, String merchantId) {
        this._id = _id;
        this.position = position;
        this.name = name;
        this.description = description;
        this.merchantId = merchantId;
    }

    public String get_id() {
        return _id;
    }

    public void set_id(String _id) {
        this._id = _id;
    }

    public Integer getPosition() {
        return position;
    }

    public void setPosition(Integer position) {
        this.position = position;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getMerchantId() {
        return merchantId;
    }

    public void setMerchantId(String merchantId) {
        this.merchantId = merchantId;
    }
}