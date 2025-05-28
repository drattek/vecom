package com.vegusa.middleware.integrations.multivende.dto;

public class OficialStore {
    private String _id;
    private String name;
    private String description;
    private String status;

    public OficialStore() {}

    public OficialStore(String _id, String name, String description, String status) {
        this._id = _id;
        this.name = name;
        this.description = description;
        this.status = status;
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

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
    }
}
