package com.vegusa.middleware.integrations.multivende.dto;

public class Pricelist {
    private String _id;
    private String name;
    private Boolean isDefault;
    private String code;
    private String description;
    private Integer position;
    private String currencyId;
    private String merchantId;
    private Currency currency;

    public static class Currency {
        private String _id;
    }

    public Pricelist() {}

    public Pricelist(String _id, String name, Boolean isDefault, String code, String description, Integer position, String currencyId, String merchantId, Currency currency) {
        this._id = _id;
        this.name = name;
        this.isDefault = isDefault;
        this.code = code;
        this.description = description;
        this.position = position;
        this.currencyId = currencyId;
        this.merchantId = merchantId;
        this.currency = currency;
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

    public Boolean getDefault() {
        return isDefault;
    }

    public void setDefault(Boolean aDefault) {
        isDefault = aDefault;
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

    public String getCurrencyId() {
        return currencyId;
    }

    public void setCurrencyId(String currencyId) {
        this.currencyId = currencyId;
    }

    public String getMerchantId() {
        return merchantId;
    }

    public void setMerchantId(String merchantId) {
        this.merchantId = merchantId;
    }

    public Currency getCurrency() {
        return currency;
    }

    public void setCurrency(Currency currency) {
        this.currency = currency;
    }
}
