package com.vegusa.middleware.integrations.jumpseller.service.common;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.entity.Category;
import com.vegusa.middleware.entity.IntegrationCategory;
import com.vegusa.middleware.integrations.jumpseller.client.common.CommonJumpsellerCient;
import com.vegusa.middleware.integrations.jumpseller.client.product.ProductJumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.dto.*;
import com.vegusa.middleware.integrations.jumpseller.entity.MapperCategory;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.repository.MapperCategoryRepository;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncProductJumpsellerRepository;
import com.vegusa.middleware.repository.CategoryRepository;
import com.vegusa.middleware.repository.IntegrationCategoryRepository;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.util.*;

@Service
public class CommonJumpsellerService {
    private final CommonJumpsellerCient commonJumpsellerCient;

    @Autowired
    private ProductJumpsellerClient jumpsellerClient;

    @Autowired
    private CategoryRepository categoryRepository;

    @Autowired
    private IntegrationCategoryRepository integrationCategoryRepository;

    @Autowired
    private MapperCategoryRepository mapperCategoryRepository;

    @Autowired
    private SyncProductJumpsellerRepository syncProductJumpsellerRepository;

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

    public void mappedBrands(List<MappedBrandDTO> data){
        for (MappedBrandDTO brand : data){
            MapperCategory item = mapperCategoryRepository.findByItemId(brand.getItemId())
                    .orElseGet(() -> {
                        MapperCategory tmp = new MapperCategory();
                        tmp.setItemId(brand.getItemId());
                        return tmp;
                    });
            item.setBrand(brand.getBrand());
            mapperCategoryRepository.save(item);
            System.out.println("Brand item: " + brand.getItemId());
        }
    }

    public void mappedCategories(Map<String, Map<String, List<MappedCatergoryDTO>>> data){
        for (Map.Entry<String, Map<String, List<MappedCatergoryDTO>>> category : data.entrySet()){
            String categoryId = category.getKey();
            Map<String, List<MappedCatergoryDTO>> categoryValue = category.getValue();

            for (Map.Entry<String, List<MappedCatergoryDTO>> subcategory : categoryValue.entrySet()){
                String subcategoryId = subcategory.getKey();
                List<MappedCatergoryDTO> subcategoryValue = subcategory.getValue();

                for (MappedCatergoryDTO item : subcategoryValue){
                    MapperCategory mapped = new MapperCategory();
                    mapped.setItemId(item.getInternal());
                    mapped.setCategoryId(categoryId);
                    if (!Objects.equals(categoryId, subcategoryId)){
                        mapped.setSubcategoryId(subcategoryId);
                    }

                    mapperCategoryRepository.save(mapped);
                    System.out.println("Sync item: " + item.getInternal());
                }
            }
        }
    }

    public Mono<Void> syncBrands(){
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts(DataArea.MSB.name());
        List<MapperCategory> categories = mapperCategoryRepository.findAll();
        Map<String, MapperCategory> mappedItems = new HashMap<>();
        for (MapperCategory category : categories){
            mappedItems.put(category.getItemId(), category);
        }

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            MapperCategory mapped = mappedItems.get(syncItem.getInternalCode());

            if (mapped != null){
                if (mapped.getBrand() != null){
                    Product product = new Product();
                    product.setPrice(syncItem.getPrice());
                    product.setName(syncItem.getName());

                    product.setBrand(mapped.getBrand());
                    JumpsellerProductDto dto = new JumpsellerProductDto(product);

                    return jumpsellerClient.updateProduct(syncItem.getResponseId(), dto)
                            .doOnSuccess(result -> {
                                Product resultProduct = result.getProduct();
                                syncItem.setBrand(resultProduct.getBrand());
                                syncProductJumpsellerRepository.save(syncItem);
                                System.out.println("Updated brand item: " + syncItem.getInternalCode());
                            })
                            .doOnError(error -> {
                                System.err.println("Error item: " + syncItem.getInternalCode());
                            });
                }
            }
            return Mono.empty();
        }).then().doOnSuccess(e -> System.out.println("Update completed"));
    }

    public Mono<Void> syncCategories(){
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts(DataArea.MSB.name());
        List<MapperCategory> categories = mapperCategoryRepository.findAll();
        Map<String, MapperCategory> mappedCategories = new HashMap<>();
        for (MapperCategory category : categories){
            mappedCategories.put(category.getItemId(), category);
        }

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            MapperCategory mapped = mappedCategories.get(syncItem.getInternalCode());

            if (mapped != null){
                Product product = new Product();
                product.setPrice(syncItem.getPrice());
                product.setName(syncItem.getName());

                List<CategoryDTO> categoryList = new ArrayList<>();
                CategoryDTO rootCategoryDTO = new CategoryDTO();
                rootCategoryDTO.setId(2185804L);
                categoryList.add(rootCategoryDTO);

                CategoryDTO mainCategoryDTO = new CategoryDTO();
                mainCategoryDTO.setId(Long.valueOf(mapped.getCategoryId()));
                categoryList.add(mainCategoryDTO);

                if (mapped.getSubcategoryId() != null){
                    CategoryDTO subcategoryDTO = new CategoryDTO();
                    subcategoryDTO.setId(Long.valueOf(mapped.getSubcategoryId()));
                    categoryList.add(subcategoryDTO);
                }

                product.setCategories(categoryList.toArray(CategoryDTO[]::new));

                JumpsellerProductDto dto = new JumpsellerProductDto(product);

                return jumpsellerClient.updateProduct(syncItem.getResponseId(), dto)
                        .doOnSuccess(result -> {
                            System.out.println("Updated item: " + syncItem.getInternalCode());
                        })
                        .doOnError(error -> {
                            System.out.println("Error item: " + syncItem.getInternalCode());
                        });
            }
            return Mono.empty();
        }).then().doOnSuccess(e -> System.out.println("Update completed"));
    }
}
