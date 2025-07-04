package com.vegusa.middleware.integrations.mercadolibre.controller.picture;

import com.vegusa.middleware.integrations.mercadolibre.service.picture.PictureMeliService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

import java.util.HashMap;

@RestController
@RequestMapping("/msb-ecommerce-middleware/mercadolibre")
public class PictureMeliController {
    @Autowired
    private PictureMeliService pictureService;

    @PostMapping("/get-image-error")
    public Mono<String> getErrors(@RequestBody HashMap<String, String> request){
        String pictureId = request.get("picture_id");
        return pictureService.getErrors(pictureId);
    }
}
