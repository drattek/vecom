package com.vegusa.middleware.integrations.jumpseller.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class CustomField {
    @JsonProperty("id")
    private Long id;

    @JsonProperty("value")
    private String value;

    @JsonProperty("variants")
    private Long[] variants;

    public CustomField() {}

    public CustomField(Long id, String value, Long[] variants) {
        this.id = id;
        this.value = value;
        this.variants = variants;
    }

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public String getValue() {
        return value;
    }

    public void setValue(String value) {
        this.value = value;
    }

    public Long[] getVariants() {
        return variants;
    }

    public void setVariants(Long[] variants) {
        this.variants = variants;
    }
}
