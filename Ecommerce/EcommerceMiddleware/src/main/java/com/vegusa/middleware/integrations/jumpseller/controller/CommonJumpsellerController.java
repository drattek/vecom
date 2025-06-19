package com.vegusa.middleware.integrations.jumpseller.controller;

import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerInfoDto;
import com.vegusa.middleware.integrations.jumpseller.dto.LanguageDto;
import com.vegusa.middleware.integrations.jumpseller.service.JumpsellerCommonService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

@RestController
@RequestMapping("msb-ecommerce-middleware/jumpseller")
public class CommonJumpsellerController {
    @Autowired
    private JumpsellerCommonService jumpsellerCommonService;

    @Autowired
    public CommonJumpsellerController() {}

    @GetMapping(value = "/app-info")
    public Mono<JumpsellerInfoDto> getAppInfo() {
        try {
            return jumpsellerCommonService.getAppInfo();
        } catch (RuntimeException e){
            System.err.println(e.getMessage());
        }
        return null;
    }

    @GetMapping(value = "/app-language")
    public Mono<LanguageDto> getLanguage(){
        try {
            return jumpsellerCommonService.getLanguage();
        } catch (RuntimeException e) {
            System.err.println(e.getMessage());
        }
        return null;
    }
}
