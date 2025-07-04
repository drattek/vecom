package com.vegusa.middleware.integrations.jumpseller.controller.image;

import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerImageDTO;
import com.vegusa.middleware.integrations.jumpseller.service.image.ImageJumpsellerService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

@RestController
@RequestMapping("msb-ecommerce-middleware/jumpseller")
public class ImageJumpsellerController {
    @Autowired
    private ImageJumpsellerService imageService;

    @GetMapping(value = "/get-images")
    public Mono<JumpsellerImageDTO[]> getProductImages(@RequestParam("product_id") String productId){
        return imageService.getProductImages(productId);
    }

    @PostMapping(value = "/download-images")
    public Mono<Void> downloadImages(){
        imageService.downloadImages().subscribe();

        return Mono.empty();
    }
}
