package com.vegusa.middleware.integrations.mercadolibre.dto.product;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class ShippingMeliDTO {
    @JsonProperty("mode")
    private String mode;

    @JsonProperty("local_pick_up")
    private Boolean localPickUp;

    @JsonProperty("free_shipping")
    private Boolean freeShipping;

    @JsonProperty("free_methods")
    private String[] freeMethods;

    public ShippingMeliDTO() {}

    public ShippingMeliDTO(String mode, Boolean localPickUp, Boolean freeShipping, String[] freeMethods) {
        this.mode = mode;
        this.localPickUp = localPickUp;
        this.freeShipping = freeShipping;
        this.freeMethods = freeMethods;
    }

    public String getMode() {
        return mode;
    }

    public void setMode(String mode) {
        this.mode = mode;
    }

    public Boolean getLocalPickUp() {
        return localPickUp;
    }

    public void setLocalPickUp(Boolean localPickUp) {
        this.localPickUp = localPickUp;
    }

    public Boolean getFreeShipping() {
        return freeShipping;
    }

    public void setFreeShipping(Boolean freeShipping) {
        this.freeShipping = freeShipping;
    }

    public String[] getFreeMethods() {
        return freeMethods;
    }

    public void setFreeMethods(String[] freeMethods) {
        this.freeMethods = freeMethods;
    }
}
