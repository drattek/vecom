package com.vegusa.middleware.integrations.multivende.dto;

public class Warranty {
    private String _id;
    private Integer position;
    private String status;
    private String name;
    private String description;
    private String content;

    public Warranty() {}

    public Warranty(String _id, Integer position, String status, String name, String description, String content) {
        this._id = _id;
        this.position = position;
        this.status = status;
        this.name = name;
        this.description = description;
        this.content = content;
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

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
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

    public String getContent() {
        return content;
    }

    public void setContent(String content) {
        this.content = content;
    }
}
