package com.vegusa.middleware.integrations.jumpseller.dto;

import com.fasterxml.jackson.annotation.JsonInclude;

public class JumpsellerProductDto {
    private Product product;

    public  JumpsellerProductDto(){}

    public JumpsellerProductDto(Product product) {
        this.product = product;
    }

    public Product getProduct() {
        return product;
    }

    public void setProduct(Product product) {
        this.product = product;
    }
}