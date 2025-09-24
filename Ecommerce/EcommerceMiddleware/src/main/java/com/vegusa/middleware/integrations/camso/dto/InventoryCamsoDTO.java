package com.vegusa.middleware.integrations.camso.dto;

import com.fasterxml.jackson.annotation.JsonAnySetter;

import java.util.HashMap;
import java.util.List;
import java.util.Map;

public class InventoryCamsoDTO {
    private Map<String, List<ItemCamsoDTO>> items;

    @JsonAnySetter
    public void addItemGroup(String key, List<ItemCamsoDTO> value) {
        if (items == null) {
            items = new HashMap<>();
        }
        items.put(key, value);
    }

    public Map<String, List<ItemCamsoDTO>> getItems() {
        return items;
    }
}
