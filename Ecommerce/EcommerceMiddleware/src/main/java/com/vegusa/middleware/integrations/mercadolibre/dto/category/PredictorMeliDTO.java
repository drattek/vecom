package com.vegusa.middleware.integrations.mercadolibre.dto.category;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class PredictorMeliDTO {
    @JsonProperty("domain_id")
    private String domainId;

    @JsonProperty("domain_name")
    private String domainName;

    @JsonProperty("category_id")
    private String categoryId;

    @JsonProperty("category_name")
    private String categoryName;

    @JsonProperty("attributes")
    private Attribute[] attributes;

    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    private static class Attribute{
        @JsonProperty("id")
        private String id;
        @JsonProperty("name")
        private String name;
        @JsonProperty("value_id")
        private String valueId;
        @JsonProperty("value_name")
        private String valueName;

        public Attribute() {}

        public Attribute(String id, String name, String valueId, String valueName) {
            this.id = id;
            this.name = name;
            this.valueId = valueId;
            this.valueName = valueName;
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
    }

    public PredictorMeliDTO() {}

    public PredictorMeliDTO(String domainId, String domainName, String categoryId, String categoryName, Attribute[] attributes) {
        this.domainId = domainId;
        this.domainName = domainName;
        this.categoryId = categoryId;
        this.categoryName = categoryName;
        this.attributes = attributes;
    }

    public String getDomainId() {
        return domainId;
    }

    public void setDomainId(String domainId) {
        this.domainId = domainId;
    }

    public String getDomainName() {
        return domainName;
    }

    public void setDomainName(String domainName) {
        this.domainName = domainName;
    }

    public String getCategoryId() {
        return categoryId;
    }

    public void setCategoryId(String categoryId) {
        this.categoryId = categoryId;
    }

    public String getCategoryName() {
        return categoryName;
    }

    public void setCategoryName(String categoryName) {
        this.categoryName = categoryName;
    }

    public Attribute[] getAttributes() {
        return attributes;
    }

    public void setAttributes(Attribute[] attributes) {
        this.attributes = attributes;
    }
}
