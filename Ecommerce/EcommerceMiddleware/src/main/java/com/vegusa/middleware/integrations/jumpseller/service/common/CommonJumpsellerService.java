package com.vegusa.middleware.integrations.jumpseller.service.common;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.entity.Category;
import com.vegusa.middleware.entity.IntegrationCategory;
import com.vegusa.middleware.integrations.jumpseller.client.common.CommonJumpsellerCient;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerInfoDto;
import com.vegusa.middleware.integrations.jumpseller.dto.LanguageDto;
import com.vegusa.middleware.repository.CategoryRepository;
import com.vegusa.middleware.repository.IntegrationCategoryRepository;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

@Service
public class CommonJumpsellerService {
    private final CommonJumpsellerCient commonJumpsellerCient;

    @Autowired
    private CategoryRepository categoryRepository;

    @Autowired
    private IntegrationCategoryRepository integrationCategoryRepository;

    @Autowired
    public CommonJumpsellerService(CommonJumpsellerCient commonJumpsellerCient) {
        this.commonJumpsellerCient = commonJumpsellerCient;
    }

    public Mono<JumpsellerInfoDto> getAppInfo(){
        return commonJumpsellerCient.getAppInfo();
    }

    public Mono<LanguageDto> getLanguage() {
        return commonJumpsellerCient.getLanguages();
    }

    public void importCategories(JSONArray categoryList){
        categoryList.forEach(item -> {
            JSONObject jumpsellerCategories = (JSONObject) item;
            Category[] categories = categoryRepository.getCategories(jumpsellerCategories.getString("name"));
            System.out.println("Importing category: " + jumpsellerCategories.getString("name"));

            for (Category category : categories){
                IntegrationCategory newCategory = new IntegrationCategory();
                newCategory.setCategoryId(category.getId().getRecId());
                newCategory.setExternalId(jumpsellerCategories.getString("jumpseller"));
                newCategory.setName(category.getId().getName());
                newCategory.setDataAreaId(DataArea.MSB.name());
                newCategory.setCompanyRefRecId(1L);
                newCategory.setIntegrationName(IntegrationType.JUMPSELLER.name());
                newCategory.setIntegrationParameterId(2L);

                integrationCategoryRepository.save(newCategory);
            }
        });
    }
}
