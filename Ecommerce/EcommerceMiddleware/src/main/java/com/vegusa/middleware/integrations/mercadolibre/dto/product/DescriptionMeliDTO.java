package com.vegusa.middleware.integrations.mercadolibre.dto.product;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class DescriptionMeliDTO {
    @JsonProperty("plain_text")
    private String plainText;

    public DescriptionMeliDTO() {}

    public DescriptionMeliDTO(String plainText) {
        this.plainText = plainText;
    }

    public String getPlainText() {
        return plainText;
    }

    public void setPlainText(String plainText) {
        this.plainText = plainText;
    }
}
