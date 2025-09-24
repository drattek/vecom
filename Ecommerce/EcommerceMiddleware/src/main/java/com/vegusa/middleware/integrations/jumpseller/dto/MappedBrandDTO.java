package com.vegusa.middleware.integrations.jumpseller.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class MappedBrandDTO {
    @JsonProperty("itemId")
    private String itemId;

    @JsonProperty("brand")
    private String brand;

    public MappedBrandDTO() {}

    public MappedBrandDTO(String itemId, String brand) {
        this.itemId = itemId;
        this.brand = brand;
    }

    public String getItemId() {
        return itemId;
    }

    public void setItemId(String itemId) {
        this.itemId = itemId;
    }

    public String getBrand() {
        return brand;
    }

    public void setBrand(String brand) {
        this.brand = brand;
    }
}
