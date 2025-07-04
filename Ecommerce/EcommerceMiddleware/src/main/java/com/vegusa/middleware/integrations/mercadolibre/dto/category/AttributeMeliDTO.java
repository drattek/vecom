package com.vegusa.middleware.integrations.mercadolibre.dto.category;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class AttributeMeliDTO {
    @JsonProperty("id")
    private String id;

    @JsonProperty("name")
    private String name;

    @JsonProperty("value_id")
    private String valueId;

    @JsonProperty("value_name")
    private String valueName;

    @JsonProperty("values")
    private AttributeValue[] values;

    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    private static class AttributeValue{
        @JsonProperty("id")
        private String id;

        @JsonProperty("name")
        private String name;

        public AttributeValue() {}

        public AttributeValue(String id, String name) {
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

    public AttributeMeliDTO() {}

    public AttributeMeliDTO(String id, String name, String valueId, String valueName, AttributeValue[] values) {
        this.id = id;
        this.name = name;
        this.valueId = valueId;
        this.valueName = valueName;
        this.values = values;
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

    public String getValueId() {
        return valueId;
    }

    public void setValueId(String valueId) {
        this.valueId = valueId;
    }

    public String getValueName() {
        return valueName;
    }

    public void setValueName(String valueName) {
        this.valueName = valueName;
    }

    public AttributeValue[] getValues() {
        return values;
    }

    public void setValues(AttributeValue[] values) {
        this.values = values;
    }
}
