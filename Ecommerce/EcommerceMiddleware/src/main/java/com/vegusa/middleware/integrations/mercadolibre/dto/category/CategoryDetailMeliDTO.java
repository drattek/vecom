package com.vegusa.middleware.integrations.mercadolibre.dto.category;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class CategoryDetailMeliDTO {
    @JsonProperty("id")
    private String id;

    @JsonProperty("name")
    private String name;

    @JsonProperty("picture")
    private String picture;

    @JsonProperty("permalink")
    private String permalink;

    @JsonProperty("total_items_in_this_category")
    private Long totalItemsInThisCategory;

    @JsonProperty("path_from_root")
    private CategoryMeliDTO[] pathFromRoot;

    @JsonProperty("children_categories")
    private CategoryMeliDTO[] childrenCategories;

    @JsonProperty("attribute_types")
    private String attributeTypes;

    public CategoryDetailMeliDTO() {}

    public CategoryDetailMeliDTO(String id, String name, String picture, String permalink, Long totalItemsInThisCategory, CategoryMeliDTO[] pathFromRoot, CategoryMeliDTO[] childrenCategories, String attributeTypes) {
        this.id = id;
        this.name = name;
        this.picture = picture;
        this.permalink = permalink;
        this.totalItemsInThisCategory = totalItemsInThisCategory;
        this.pathFromRoot = pathFromRoot;
        this.childrenCategories = childrenCategories;
        this.attributeTypes = attributeTypes;
    }

    public String getId() {
        return id;
    }

    public void setId(String id) {
        this.id = id;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getPicture() {
        return picture;
    }

    public void setPicture(String picture) {
        this.picture = picture;
    }

    public String getPermalink() {
        return permalink;
    }

    public void setPermalink(String permalink) {
        this.permalink = permalink;
    }

    public Long getTotalItemsInThisCategory() {
        return totalItemsInThisCategory;
    }

    public void setTotalItemsInThisCategory(Long totalItemsInThisCategory) {
        this.totalItemsInThisCategory = totalItemsInThisCategory;
    }

    public CategoryMeliDTO[] getPathFromRoot() {
        return pathFromRoot;
    }

    public void setPathFromRoot(CategoryMeliDTO[] pathFromRoot) {
        this.pathFromRoot = pathFromRoot;
    }

    public CategoryMeliDTO[] getChildrenCategories() {
        return childrenCategories;
    }

    public void setChildrenCategories(CategoryMeliDTO[] childrenCategories) {
        this.childrenCategories = childrenCategories;
    }

    public String getAttributeTypes() {
        return attributeTypes;
    }

    public void setAttributeTypes(String attributeTypes) {
        this.attributeTypes = attributeTypes;
    }
}
