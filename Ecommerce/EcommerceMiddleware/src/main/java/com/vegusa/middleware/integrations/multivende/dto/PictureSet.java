package com.vegusa.middleware.integrations.multivende.dto;

public class PictureSet {
    private String _id;
    private String branch;
    private String name;
    private String description;
    private String merchantId;

    public PictureSet() {}

    public PictureSet(String _id, String branch, String name, String description, String merchantId) {
        this._id = _id;
        this.branch = branch;
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

    public String getBranch() {
        return branch;
    }

    public void setBranch(String branch) {
        this.branch = branch;
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
