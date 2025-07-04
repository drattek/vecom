package com.vegusa.middleware.integrations.jumpseller.service.category;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.entity.IntegrationCategory;
import com.vegusa.middleware.integrations.jumpseller.client.category.CategoryJumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.dto.Category;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerCategoryDto;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncCategoryRepositoryJumpseller;
import com.vegusa.middleware.integrations.jumpseller.utils.CategoryUtils;
import com.vegusa.middleware.repository.IntegrationCategoryRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

import java.util.List;

@Service
public class CategoryJumpsellerService {

    private final CategoryJumpsellerClient categoryJumpsellerClient;

    @Autowired
    private CategoryUtils categoryUtils;

    @Autowired
    private SyncCategoryRepositoryJumpseller syncCategoryRepository;
    private final IntegrationCategoryRepository integrationCategoryRepository;

    @Autowired
    public CategoryJumpsellerService(CategoryJumpsellerClient categoryJumpsellerClient,
                                     IntegrationCategoryRepository integrationCategoryRepository){
        this.categoryJumpsellerClient = categoryJumpsellerClient;
        this.integrationCategoryRepository = integrationCategoryRepository;
    }

    public Mono<JumpsellerCategoryDto> getCategoryById(long id) {
        return categoryJumpsellerClient.getCategoryById(id);
    }

    public Mono<JumpsellerCategoryDto[]> getAllCategories(){
        return categoryJumpsellerClient.getAllCategories()
                .doOnNext(categories -> {
                    for (JumpsellerCategoryDto category : categories) {
                        Category jumpsellerCategory = category.getCategory();
                        List<IntegrationCategory> localCategories = integrationCategoryRepository.getCategories(
                                jumpsellerCategory.getId().toString(),
                                DataArea.MSB.name(),
                                IntegrationType.JUMPSELLER.name()
                        );

                        for (IntegrationCategory local : localCategories){
                            System.out.println("Updating category: " + local.getName() + " with: " + jumpsellerCategory.getName());
                            local.setExternalName(jumpsellerCategory.getName());
                            integrationCategoryRepository.save(local);
                        }
                    }
                })
                .doOnSuccess(categories -> System.out.println("Categories sync ended"));
    }

    public Mono<JumpsellerCategoryDto> createCategory(JumpsellerCategoryDto category){
        return categoryJumpsellerClient.createCategory(category);
    }

    public Mono<JumpsellerCategoryDto> updateCategory(long id, JumpsellerCategoryDto category){
        return categoryJumpsellerClient.updateCategory(id, category);
    }

    public Mono<Void> deleteCategory(long id){
        return categoryJumpsellerClient.deleteCategory(id);
    }
}
