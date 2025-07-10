package com.vegusa.middleware.integrations.jumpseller.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class JumpsellerCustomFieldDTO {
    @JsonProperty("field")
    private CustomField field;

    public JumpsellerCustomFieldDTO() {}

    public JumpsellerCustomFieldDTO(CustomField field) {
        this.field = field;
    }

    public CustomField getField() {
        return field;
    }

    public void setField(CustomField field) {
        this.field = field;
    }
}
