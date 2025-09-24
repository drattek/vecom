package com.vegusa.middleware.integrations.mercadolibre.controller.moderation;

import com.vegusa.middleware.integrations.mercadolibre.service.moderation.ModerationMeliService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

@RestController
@RequestMapping("/msb-ecommerce-middleware/mercadolibre")
public class ModerationMeliController {
    @Autowired
    private ModerationMeliService moderationService;

    @GetMapping(value = "/get-moderation")
    public Mono<String> getModeration(@RequestParam("item_id") String itemId){
        return moderationService.getModeration(itemId);
    }
}
