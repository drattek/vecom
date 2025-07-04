package com.vegusa.middleware.integrations.mercadolibre.dto.category;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class CategoryMeliDTO {
    @JsonProperty("id")
    private String id;

    @JsonProperty("name")
    private String name;

    public CategoryMeliDTO() {}

    public CategoryMeliDTO(String id, String name) {
        this.id = id;
        this.name = name;
    }

    public String getId() {
        return id;
    }

    public void setId(String id) {
        this.id = id;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }
}
