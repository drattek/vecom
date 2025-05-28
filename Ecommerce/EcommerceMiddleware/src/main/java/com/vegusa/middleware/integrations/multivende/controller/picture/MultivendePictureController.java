package com.vegusa.middleware.integrations.multivende.controller.picture;

import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.PictureSet;
import com.vegusa.middleware.integrations.multivende.service.picture.PictureSetService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

@RestController
@RequestMapping("msb-ecommerce-middleware/multivende")
public class MultivendePictureController {
    @Autowired
    private PictureSetService pictureSetService;

    @GetMapping(value = "/get-picture-set")
    public Mono<EntriesDto<PictureSet>> getAllPictureSet(){
        return pictureSetService.getAllPictureSet();
    }
}
