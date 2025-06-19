package com.vegusa.middleware.dto;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import jakarta.validation.constraints.NotBlank;

@JsonIgnoreProperties(ignoreUnknown = true)
public class Request {
    @NotBlank(message = "dataAreaId es required")
    private String dataAreaId;

    public Request() {}

    public Request(String dataAreaId) {
        this.dataAreaId = dataAreaId;
    }

    public String getDataAreaId() {
        return dataAreaId;
    }

    public void setDataAreaId(String dataAreaId) {
        this.dataAreaId = dataAreaId;
    }
}
