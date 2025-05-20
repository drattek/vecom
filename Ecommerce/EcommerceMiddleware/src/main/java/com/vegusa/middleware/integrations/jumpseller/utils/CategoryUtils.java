package com.vegusa.middleware.integrations.jumpseller.utils;

import com.vegusa.middleware.entity.SyncCategory;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerCategoryDto;
import org.springframework.stereotype.Component;

@Component
public class CategoryUtils {

    public SyncCategory toEntity (JumpsellerCategoryDto dto) {
        SyncCategory entity = new SyncCategory();
        entity.setResponseId(String.valueOf(dto.getCategory().getId()));
        entity.setName(dto.getCategory().getName());
        entity.setBranch(String.valueOf(dto.getCategory().getParent_id()));
        entity.setDescription(dto.getCategory().getDescription());
        entity.setResponseStatus("created");
        entity.setIntegrationCompany("JUMPSELLER");

        return entity;
    }
}
