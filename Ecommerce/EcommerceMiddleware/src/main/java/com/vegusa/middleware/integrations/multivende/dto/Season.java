package com.vegusa.middleware.integrations.multivende.dto;

public class Season {
    private String _id;
    private String name;
    private String description;
    private String merchantId;

    public Season() {}

    public Season(String _id, String name, String description, String merchantId) {
        this._id = _id;
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
