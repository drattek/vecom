package com.vegusa.middleware.integrations.jumpseller.dto;

public class JumpsellerCategoryDto {
    private CategoryDTO categoryDTO;

    public  JumpsellerCategoryDto(){}

    public JumpsellerCategoryDto(CategoryDTO categoryDTO) {
        this.categoryDTO = categoryDTO;
    }

    public CategoryDTO getCategory() {
        return categoryDTO;
    }

    public void setCategory(CategoryDTO categoryDTO) {
        this.categoryDTO = categoryDTO;
    }
}
