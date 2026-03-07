package com.vegusa.middleware.integrations.mercadolibre.service.common;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.entity.Category;
import com.vegusa.middleware.entity.IntegrationCategory;
import com.vegusa.middleware.entity.ProductCategory;
import com.vegusa.middleware.entity.SyncItem;
import com.vegusa.middleware.integrations.mercadolibre.client.common.CommonMeliClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.common.CurrencyMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.entity.SyncItemMeli;
import com.vegusa.middleware.integrations.mercadolibre.oauth.TokenStorageMeli;
import com.vegusa.middleware.integrations.mercadolibre.repository.SyncItemMeliRepository;
import com.vegusa.middleware.repository.local.CategoryRepository;
import com.vegusa.middleware.repository.local.IntegrationCategoryRepository;
import com.vegusa.middleware.repository.local.ProductCategoryRepository;
import com.vegusa.middleware.repository.local.SyncItemRepository;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.util.HashMap;

@Service
public class CommonMeliService {
    @Autowired
    private CommonMeliClient client;

    @Autowired
    private SyncItemRepository syncItemRepository;

    @Autowired
    private SyncItemMeliRepository syncItemMeliRepository;

    @Autowired
    private CategoryRepository categoryRepository;

    @Autowired
    private IntegrationCategoryRepository integrationCategoryRepository;

    @Autowired
    private ProductCategoryRepository productCategoryRepository;

    @Autowired
    private TokenStorageMeli tokenStorage;

    public Mono<CurrencyMeliDTO[]> getCurrencies(){
        return client.getCurrencies();
    }

    public Mono<Void> importProducts(JSONArray productList, String dataAreaId){
        HashMap<String, JSONObject> itemMap = getItems(productList);
        SyncItem[] syncItems = syncItemRepository.getSyncItem();

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            JSONObject product = itemMap.get(syncItem.getCode());

            if (product != null){
                SyncItemMeli product_entity = new SyncItemMeli();
                product_entity.setResponseId(product.getString("meli_id"));
                product_entity.setInternalCode(syncItem.getInternalCode());
                product_entity.setCode(product.getString("sku"));
                product_entity.setName(product.getString("name"));
                product_entity.setDescription(syncItem.getDescription());
                product_entity.setPrice(product.getBigDecimal("price"));
                product_entity.setCurrencyId(product.getString("currency"));
                product_entity.setDataAreaId(dataAreaId);
                product_entity.setCompanyRefRecId(1L);

                syncItemMeliRepository.save(product_entity);
            }

            return Mono.empty();
        }).then().doOnSuccess(e -> {
            System.out.println("Import completed");
        });
    }

    public void importCategories(JSONArray categoryList, String dataAreaId){
        categoryList.forEach(item -> {
            JSONObject meliCategories = (JSONObject) item;
            Category[] categories = categoryRepository.getCategories(meliCategories.getString("name"));
            System.out.println("Importing category: " + meliCategories.getString("name"));

            for (Category category : categories) {
                IntegrationCategory newCategory = new IntegrationCategory();
                newCategory.setCategoryId(category.getId().getRecId());
                newCategory.setExternalId(meliCategories.getString("meli_id"));
                newCategory.setName(category.getId().getName());
                newCategory.setDataAreaId(dataAreaId);
                newCategory.setCompanyRefRecId(1L);
                newCategory.setIntegrationName(IntegrationType.MERCADO_LIBRE.name());
                newCategory.setIntegrationParameterId(tokenStorage.getID());

                integrationCategoryRepository.save(newCategory);
            }
        });
    }

    public void categoryCorrection(){
        SyncItemMeli[] syncItems = syncItemMeliRepository.getItems("MSB");

        for (SyncItemMeli syncItem : syncItems){
            ProductCategory category = productCategoryRepository.getProductCategory(syncItem.getInternalCode(), "MSB");
            IntegrationCategory syncCategory = integrationCategoryRepository.getCategory(category.getCategory().getId().getRecId(), DataArea.MSB.name(), IntegrationType.MERCADO_LIBRE.name());

            syncItem.setCategoryId(syncCategory.getExternalId());
            syncItemMeliRepository.save(syncItem);
            System.out.println("Updated " + syncItem.getInternalCode() + " with category " + syncCategory.getExternalId());
        }
    }

    private HashMap<String, JSONObject> getItems(JSONArray items){
        HashMap<String, JSONObject> response = new HashMap<>();

        for (int i = 0; i < items.length(); i++){
            JSONObject obj = items.getJSONObject(i);
            String code = obj.getString("sku");
            response.put(code, obj);
        }

        return response;
    }
}
