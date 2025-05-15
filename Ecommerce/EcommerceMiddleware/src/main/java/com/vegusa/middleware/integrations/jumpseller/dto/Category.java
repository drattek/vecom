package com.vegusa.middleware.integrations.jumpseller.dto;

public class Category {
    private long id;
    private String name;
    private String description;
    private long parent_id;
    private String permalink;

    public  Category(){}

    public Category(long id, String name, String description, long parent_id, String permalink) {
        this.id = id;
        this.name = name;
        this.description = description;
        this.parent_id = parent_id;
        this.permalink = permalink;
    }

    public long getId() {
        return id;
    }

    public void setId(long id) {
        this.id = id;
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

    public long getParent_id() {
        return parent_id;
    }

    public void setParent_id(long parent_id) {
        this.parent_id = parent_id;
    }

    public String getPermalink() {
        return permalink;
    }

    public void setPermalink(String permalink) {
        this.permalink = permalink;
    }
}
