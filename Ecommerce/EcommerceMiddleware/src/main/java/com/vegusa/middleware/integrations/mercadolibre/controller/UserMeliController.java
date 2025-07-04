package com.vegusa.middleware.integrations.mercadolibre.controller;

import com.vegusa.middleware.integrations.mercadolibre.dto.user.UserMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.service.user.UserMeliService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

@RestController
@RequestMapping("msb-ecommerce-middleware/mercadolibre")
public class UserMeliController {
    @Autowired
    private UserMeliService userService;

    @GetMapping(value = "/user-me")
    public Mono<UserMeliDTO> getUserMe(){
        return userService.getUserMe();
    }

    @GetMapping(value = "/test-user")
    public Mono<String> getTestUser(){
        return userService.getTestUser();
    }
}
