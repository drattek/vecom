package com.vegusa.middleware.integrations.multivende.dto;

import com.fasterxml.jackson.databind.annotation.JsonDeserialize;
import com.vegusa.middleware.integrations.multivende.utils.ImageReferenceDeserializer;

import java.util.List;

public class UploadImageVersionDto {
    private String productVersionId;

    @JsonDeserialize(using = ImageReferenceDeserializer.class)
    private List<ImageReference> images;

    public UploadImageVersionDto() {}

    public UploadImageVersionDto(String productVersionId, List<ImageReference> images) {
        this.productVersionId = productVersionId;
        this.images = images;
    }

    public String getProductVersionId() {
        return productVersionId;
    }

    public void setProductVersionId(String productId) {
        this.productVersionId = productId;
    }

    public List<ImageReference> getImages() {
        return images;
    }

    public void setImages(List<ImageReference> images) {
        this.images = images;
    }
}
