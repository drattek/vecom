package com.vegusa.middleware.integrations.jumpseller.dto;

public class JumpsellerCategoryDto {
    private Category category;

    public  JumpsellerCategoryDto(){}

    public JumpsellerCategoryDto(Category category) {
        this.category = category;
    }

    public Category getCategory() {
        return category;
    }

    public void setCategory(Category category) {
        this.category = category;
    }
}
