package com.vegusa.middleware.integrations.multivende.dto;

import com.fasterxml.jackson.databind.annotation.JsonDeserialize;
import com.vegusa.middleware.integrations.multivende.utils.ImageReferenceDeserializer;

import java.util.List;

public class UploadImageDto {
    private String productId;

    @JsonDeserialize(using = ImageReferenceDeserializer.class)
    private List<ImageReference> images;

    public UploadImageDto() {}

    public UploadImageDto(String productId, List<ImageReference> images) {
        this.productId = productId;
        this.images = images;
    }

    public String getProductId() {
        return productId;
    }

    public void setProductId(String productId) {
        this.productId = productId;
    }

    public List<ImageReference> getImages() {
        return images;
    }

    public void setImages(List<ImageReference> images) {
        this.images = images;
    }
}
