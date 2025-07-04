package com.vegusa.middleware.integrations.mercadolibre.dto.publication;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class PublicationAvailableMeliDTO {
    @JsonProperty("category_id")
    private String categoryId;

    @JsonProperty("available")
    private PublicationTypeMeliDTO[] available;

    public String getCategoryId() {
        return categoryId;
    }

    public void setCategoryId(String categoryId) {
        this.categoryId = categoryId;
    }

    public PublicationTypeMeliDTO[] getAvailable() {
        return available;
    }

    public void setAvailable(PublicationTypeMeliDTO[] available) {
        this.available = available;
    }
}
