package com.vegusa.middleware.integrations.jumpseller.service;

import com.vegusa.middleware.integrations.jumpseller.client.common.JumpsellerCommon;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerInfoDto;
import com.vegusa.middleware.integrations.jumpseller.dto.LanguageDto;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

@Service
public class JumpsellerCommonService {
    private JumpsellerCommon jumpsellerCommon;

    @Autowired
    public JumpsellerCommonService(JumpsellerCommon jumpsellerCommon) {
        this.jumpsellerCommon = jumpsellerCommon;
    }

    public Mono<JumpsellerInfoDto> getAppInfo(){
        return jumpsellerCommon.getAppInfo();
    }

    public Mono<LanguageDto> getLanguage() {
        return jumpsellerCommon.getLanguages();
    }
}
