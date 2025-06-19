package com.vegusa.middleware.integrations.multivende.service.picture;

import com.vegusa.middleware.integrations.multivende.client.picture.MultivendeProductPicture;
import com.vegusa.middleware.integrations.multivende.dto.UploadImageDto;
import com.vegusa.middleware.integrations.multivende.dto.UploadImageResponse;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

@Service
public class ProductPictureService {
    private final MultivendeProductPicture multivendeProductPicture;

    public ProductPictureService(MultivendeProductPicture multivendeProductPicture) {
        this.multivendeProductPicture = multivendeProductPicture;
    }

    public Mono<UploadImageResponse[]> uploadImages(String albumId){
        UploadImageDto[] products = new UploadImageDto[0];
        return multivendeProductPicture.uploadImageByUrl(products, albumId);
    }
}
