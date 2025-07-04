package com.vegusa.middleware.dto;

import com.vegusa.middleware.entity.ProductCategories;
import com.vegusa.middleware.entity.ProductImage;
import com.vegusa.middleware.entity.Products;

import java.math.BigDecimal;
import java.util.List;

public class ProductInfo {
    private Products product;
    private ProductCategories category;
    private BigDecimal stock;
    private BigDecimal price;
    private List<ProductImage> images;

    public ProductInfo() {}

    public ProductInfo(Products product, ProductCategories category, BigDecimal stock, BigDecimal price, List<ProductImage> images) {
        this.product = product;
        this.category = category;
        this.stock = stock;
        this.price = price;
        this.images = images;
    }

    public Products getProduct() {
        return product;
    }

    public void setProduct(Products product) {
        this.product = product;
    }

    public ProductCategories getCategory() {
        return category;
    }

    public void setCategory(ProductCategories category) {
        this.category = category;
    }

    public BigDecimal getStock() {
        return stock;
    }

    public void setStock(BigDecimal stock) {
        this.stock = stock;
    }

    public BigDecimal getPrice() {
        return price;
    }

    public void setPrice(BigDecimal price) {
        this.price = price;
    }

    public List<ProductImage> getImages() {
        return images;
    }

    public void setImages(List<ProductImage> images) {
        this.images = images;
    }
}
