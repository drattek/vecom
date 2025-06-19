package com.vegusa.middleware.integrations.jumpseller.dto;

public class StockDto {
    private Long location_id;
    private Long product_id;
    private Long variant_id;
    private Boolean stock_unlimited;
    private Long stock;

    public StockDto() {}

    public StockDto(Long location_id, Long product_id, Long variant_id, Boolean stock_unlimited, Long stock) {
        this.location_id = location_id;
        this.product_id = product_id;
        this.variant_id = variant_id;
        this.stock_unlimited = stock_unlimited;
        this.stock = stock;
    }

    public Long getLocation_id() {
        return location_id;
    }

    public void setLocation_id(Long location_id) {
        this.location_id = location_id;
    }

    public Long getProduct_id() {
        return product_id;
    }

    public void setProduct_id(Long product_id) {
        this.product_id = product_id;
    }

    public Long getVariant_id() {
        return variant_id;
    }

    public void setVariant_id(Long variant_id) {
        this.variant_id = variant_id;
    }

    public Boolean getStock_unlimited() {
        return stock_unlimited;
    }

    public void setStock_unlimited(Boolean stock_unlimited) {
        this.stock_unlimited = stock_unlimited;
    }

    public Long getStock() {
        return stock;
    }

    public void setStock(Long stock) {
        this.stock = stock;
    }
}
