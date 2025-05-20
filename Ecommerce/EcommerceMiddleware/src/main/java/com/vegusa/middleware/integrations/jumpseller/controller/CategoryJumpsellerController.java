package com.vegusa.middleware.integrations.jumpseller.controller;

import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerCategoryDto;
import com.vegusa.middleware.integrations.jumpseller.service.JumpsellerCategoryService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

@RestController
@RequestMapping("msb-ecommerce-middleware/jumpseller")
public class CategoryJumpsellerController {

    @Autowired
    JumpsellerCategoryService jumpsellerCategoryService;

    @Autowired
    public CategoryJumpsellerController(){}

    @GetMapping(value = "/get-categories")
    public Mono<JumpsellerCategoryDto[]> getCategories (){
        try {
            return jumpsellerCategoryService.getAllCategories();
        } catch (RuntimeException e) {
            System.err.println(e.getMessage());
        }
        return null;
    }
}
