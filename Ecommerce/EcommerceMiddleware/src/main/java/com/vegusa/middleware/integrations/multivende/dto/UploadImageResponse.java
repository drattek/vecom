package com.vegusa.middleware.integrations.multivende.dto;

public class UploadImageResponse {
    private String productId;
    private PictureProduct[] imagesProcess;

    public UploadImageResponse() {}

    public UploadImageResponse(String productId, PictureProduct[] imagesProcess) {
        this.productId = productId;
        this.imagesProcess = imagesProcess;
    }

    public String getProductId() {
        return productId;
    }

    public void setProductId(String productId) {
        this.productId = productId;
    }

    public PictureProduct[] getImagesProcess() {
        return imagesProcess;
    }

    public void setImagesProcess(PictureProduct[] imagesProcess) {
        this.imagesProcess = imagesProcess;
    }
}
