package com.vegusa.middleware.entity;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.databind.JsonNode;

public class SyncItemInventoryReader {
    @JsonProperty("code")
    private String code;
    @JsonProperty("_id")
    private String idEcom;
    @JsonProperty("WarehouseId")
    private String warehouseId;
    @JsonProperty("productRelocation")
    private JsonNode productRelocation;
    @JsonProperty("availableProductStock")
    private JsonNode availableProductStock;
    @JsonProperty("success")
    private String success;
    @JsonProperty("amount")
    private String amount;
    @JsonProperty("error")
    private String error;

    public String getCode(){
        return code;
    }
    public String getIdEcom(){
        return idEcom;
    }
    public String getWarehouseId(){
        return warehouseId;
    }
    public JsonNode getProductRelocation(){
        return productRelocation;
    }
    public JsonNode getAvailableProductStock(){
        return availableProductStock;
    }
    public String getSuccess(){
        return success;
    }
    public String getAmount(){
        return amount;
    }
    public String getError(){
        return error;
    }
}
