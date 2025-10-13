package com.vegusa.middleware.integrations.jumpseller.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class Field {
    @JsonProperty("id")
    private String id;

    @JsonProperty("custom_field_id")
    private String customFieldId;

    @JsonProperty("type")
    private String type;

    @JsonProperty("label")
    private String label;

    @JsonProperty("value_id")
    private String valueId;

    @JsonProperty("value")
    private String value;

    @JsonProperty("variant_id")
    private String variantId;

    public Field() {}

    public Field(String id, String customFieldId, String type, String label, String valueId, String value, String variantId) {
        this.id = id;
        this.customFieldId = customFieldId;
        this.type = type;
        this.label = label;
        this.valueId = valueId;
        this.value = value;
        this.variantId = variantId;
    }

    public String getId() {
        return id;
    }

    public void setId(String id) {
        this.id = id;
    }

    public String getCustomFieldId() {
        return customFieldId;
    }

    public void setCustomFieldId(String customFieldId) {
        this.customFieldId = customFieldId;
    }

    public String getType() {
        return type;
    }

    public void setType(String type) {
        this.type = type;
    }

    public String getLabel() {
        return label;
    }

    public void setLabel(String label) {
        this.label = label;
    }

    public String getValueId() {
        return valueId;
    }

    public void setValueId(String valueId) {
        this.valueId = valueId;
    }

    public String getValue() {
        return value;
    }

    public void setValue(String value) {
        this.value = value;
    }

    public String getVariantId() {
        return variantId;
    }

    public void setVariantId(String variantId) {
        this.variantId = variantId;
    }
}
