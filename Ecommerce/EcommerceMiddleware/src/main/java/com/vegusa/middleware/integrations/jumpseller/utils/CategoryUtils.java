package com.vegusa.middleware.integrations.jumpseller.utils;

import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerCategoryDto;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncCategoryJumpseller;
import org.springframework.stereotype.Component;

@Component
public class CategoryUtils {

    public SyncCategoryJumpseller toEntity (JumpsellerCategoryDto dto) {
        SyncCategoryJumpseller entity = new SyncCategoryJumpseller();
        entity.setResponseId(String.valueOf(dto.getCategory().getId()));
        entity.setName(dto.getCategory().getName());
        entity.setBranch(String.valueOf(dto.getCategory().getParent_id()));
        entity.setDescription(dto.getCategory().getDescription());
        entity.setResponseStatus("created");
        entity.setIntegrationCompany("JUMPSELLER");
        entity.setDataAreaId("MSB");
        entity.setCompanyRefRecId(1L);

        return entity;
    }
}
