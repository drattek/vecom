package com.vegusa.middleware.integrations.jumpseller.service;

import com.vegusa.middleware.integrations.jumpseller.client.category.JumpsellerCategory;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerCategoryDto;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncCategoryJumpseller;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncCategoryRepositoryJumpseller;
import com.vegusa.middleware.integrations.jumpseller.utils.CategoryUtils;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

@Service
public class JumpsellerCategoryService {

    private final JumpsellerCategory jumpsellerCategory;

    @Autowired
    private CategoryUtils categoryUtils;

    @Autowired
    private SyncCategoryRepositoryJumpseller syncCategoryRepository;

    @Autowired
    public JumpsellerCategoryService(JumpsellerCategory jumpsellerCategory){
        this.jumpsellerCategory = jumpsellerCategory;
    }

    public Mono<JumpsellerCategoryDto> getCategoryById(long id) {
        return jumpsellerCategory.getCategoryById(id);
    }

    public Mono<JumpsellerCategoryDto[]> getAllCategories(){
        return jumpsellerCategory.getAllCategories()
                .doOnNext(categories -> {
                    for (JumpsellerCategoryDto category : categories) {
                        SyncCategoryJumpseller categoryEntity = categoryUtils.toEntity(category);
                        syncCategoryRepository.save(categoryEntity);
                    }
                })
                .doOnSuccess(categories -> {
                    System.out.println("Categories sync ended");
                });
    }

    public Mono<JumpsellerCategoryDto> createCategory(JumpsellerCategoryDto category){
        return jumpsellerCategory.createCategory(category);
    }

    public Mono<JumpsellerCategoryDto> updateCategory(long id, JumpsellerCategoryDto category){
        return jumpsellerCategory.updateCategory(id, category);
    }

    public Mono<Void> deleteCategory(long id){
        return jumpsellerCategory.deleteCategory(id);
    }
}
