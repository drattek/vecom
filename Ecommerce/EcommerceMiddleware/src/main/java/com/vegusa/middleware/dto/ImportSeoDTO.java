package com.vegusa.middleware.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class ImportSeoDTO {
    @JsonProperty("code")
    private String code;
    @JsonProperty("internal")
    private String internal;
    @JsonProperty("title")
    private String title;
    @JsonProperty("description")
    private String description;

    public ImportSeoDTO() {}

    public ImportSeoDTO(String code, String internal, String title, String description) {
        this.code = code;
        this.internal = internal;
        this.title = title;
        this.description = description;
    }

    public String getCode() {
        return code;
    }

    public void setCode(String code) {
        this.code = code;
    }

    public String getInternal() {
        return internal;
    }

    public void setInternal(String internal) {
        this.internal = internal;
    }

    public String getTitle() {
        return title;
    }

    public void setTitle(String title) {
        this.title = title;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }
}
