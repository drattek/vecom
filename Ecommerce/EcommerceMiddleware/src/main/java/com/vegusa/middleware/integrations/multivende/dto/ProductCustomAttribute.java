package com.vegusa.middleware.integrations.multivende.dto;

public class ProductCustomAttribute {
    private String _id;
    private String name;
    private String code;
    private String description;
    private Integer position;
    private String status;
    private String customAttributeSetId;
    private String customAttributeScopeId;
    private String customAttributeTypeId;

    public ProductCustomAttribute() {}

    public ProductCustomAttribute(String _id, String name, String code, String description, Integer position, String status, String customAttributeSetId, String customAttributeScopeId, String customAttributeTypeId) {
        this._id = _id;
        this.name = name;
        this.code = code;
        this.description = description;
        this.position = position;
        this.status = status;
        this.customAttributeSetId = customAttributeSetId;
        this.customAttributeScopeId = customAttributeScopeId;
        this.customAttributeTypeId = customAttributeTypeId;
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

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
    }

    public String getCustomAttributeSetId() {
        return customAttributeSetId;
    }

    public void setCustomAttributeSetId(String customAttributeSetId) {
        this.customAttributeSetId = customAttributeSetId;
    }

    public String getCustomAttributeScopeId() {
        return customAttributeScopeId;
    }

    public void setCustomAttributeScopeId(String customAttributeScopeId) {
        this.customAttributeScopeId = customAttributeScopeId;
    }

    public String getCustomAttributeTypeId() {
        return customAttributeTypeId;
    }

    public void setCustomAttributeTypeId(String customAttributeTypeId) {
        this.customAttributeTypeId = customAttributeTypeId;
    }
}