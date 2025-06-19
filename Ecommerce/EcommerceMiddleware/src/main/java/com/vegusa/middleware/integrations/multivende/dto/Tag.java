package com.vegusa.middleware.integrations.multivende.dto;

public class Tag {
    private String _id;
    private String name;
    private String slug;
    private String description;
    private String status;

    public Tag() {}

    public Tag(String _id, String name, String slug, String description, String status) {
        this._id = _id;
        this.name = name;
        this.slug = slug;
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

    public String getSlug() {
        return slug;
    }

    public void setSlug(String slug) {
        this.slug = slug;
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
