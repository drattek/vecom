package com.vegusa.middleware.integrations.camso.dto;

import com.fasterxml.jackson.annotation.JsonAnySetter;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.math.BigDecimal;
import java.util.HashMap;
import java.util.Map;
import java.util.Set;

public class ItemCamsoDTO {
    @JsonProperty("Grupo")
    private String group;
    @JsonProperty("ItemCode")
    private String itemCode;
    @JsonProperty("ItemName")
    private String itemName;
    @JsonProperty("Price")
    private String price;
    @JsonProperty("Clasgral")
    private String clasgral;
    @JsonProperty("Marca")
    private String brand;
    @JsonProperty("Currency")
    private String currency;
    private Map<String, BigDecimal> locations = new HashMap<>();

    public String getGroup() {
        return group;
    }

    public void setGroup(String group) {
        this.group = group;
    }

    public String getItemCode() {
        return itemCode;
    }

    public void setItemCode(String itemCode) {
        this.itemCode = itemCode;
    }

    public String getItemName() {
        return itemName;
    }

    public void setItemName(String itemName) {
        this.itemName = itemName;
    }

    public String getPrice() {
        return price;
    }

    public void setPrice(String price) {
        this.price = price;
    }

    public String getClasgral() {
        return clasgral;
    }

    public void setClasgral(String clasgral) {
        this.clasgral = clasgral;
    }

    public String getBrand() {
        return brand;
    }

    public void setBrand(String brand) {
        this.brand = brand;
    }

    public String getCurrency() {
        return currency;
    }

    public void setCurrency(String currency) {
        this.currency = currency;
    }

    @JsonAnySetter
    public void handleUnknown(String key, Object value) {
        Set<String> fixedKeys = Set.of("Grupo", "ItemCode", "ItemName", "Price", "Clasgral", "Marca", "Currency");

        if (!fixedKeys.contains(key)) {
            try {
                if (value instanceof Number) {
                    locations.put(key, new BigDecimal(value.toString()));
                } else if (value instanceof String) {
                    locations.put(key, new BigDecimal((String) value));
                }
            } catch (NumberFormatException e) {
                System.err.println("No se pudo convertir el valor de " + key + " a BigDecimal: " + value);
            }
        }
    }

    public Map<String, BigDecimal> getLocations() {
        return locations;
    }
}