package com.vegusa.middleware.integrations.jumpseller.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class MappedCatergoryDTO {
    @JsonProperty("internal")
    private String internal;

    @JsonProperty("partNumber")
    private String partNumber;

    public MappedCatergoryDTO() {}

    public MappedCatergoryDTO(String internal, String partNumber) {
        this.internal = internal;
        this.partNumber = partNumber;
    }

    public String getInternal() {
        return internal;
    }

    public void setInternal(String internal) {
        this.internal = internal;
    }

    public String getPartNumber() {
        return partNumber;
    }

    public void setPartNumber(String partNumber) {
        this.partNumber = partNumber;
    }
}
