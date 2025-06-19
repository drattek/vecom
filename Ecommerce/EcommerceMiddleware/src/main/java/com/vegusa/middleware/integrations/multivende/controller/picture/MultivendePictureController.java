package com.vegusa.middleware.integrations.multivende.controller.picture;

import com.vegusa.middleware.entity.Company;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.PictureSet;
import com.vegusa.middleware.integrations.multivende.service.picture.PictureSetService;
import com.vegusa.middleware.integrations.multivende.service.picture.ProductPictureService;
import com.vegusa.middleware.utils.CommonUtils;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

import java.util.HashMap;

@RestController
@RequestMapping("msb-ecommerce-middleware/multivende")
public class MultivendePictureController {
    @Autowired
    private PictureSetService pictureSetService;

    @Autowired
    private ProductPictureService productPictureService;

    @Autowired
    private CommonUtils commonUtils;

    @GetMapping(value = "/get-picture-set")
    public Mono<EntriesDto<PictureSet>> getAllPictureSet(){
        return pictureSetService.getAllPictureSet();
    }

    @PostMapping(value = "/upload-images")
    public Mono<ResponseEntity<String>> uploadImages(
            @RequestParam(defaultValue = "MSB") String dataAreaId,
            @RequestParam(defaultValue = "default") String albumId,
            @RequestParam(defaultValue = "100") int itemsPerCall
    ){
        Company company = commonUtils.getCompany(dataAreaId);
        productPictureService.uploadImages(albumId).subscribe();

        return Mono.just(ResponseEntity.accepted().body("Uploading in process"));
    }
}
