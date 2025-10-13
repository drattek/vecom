package com.vegusa.middleware.integrations.jumpseller.service.common;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.constants.ProductInterface;
import com.vegusa.middleware.dto.ImportProductDTO;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.integrations.jumpseller.client.common.CommonJumpsellerCient;
import com.vegusa.middleware.integrations.jumpseller.client.image.ImageJumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.client.product.ProductJumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.dto.*;
import com.vegusa.middleware.integrations.jumpseller.entity.MapperCategory;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.repository.MapperCategoryRepository;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncProductJumpsellerRepository;
import com.vegusa.middleware.repository.*;
import com.vegusa.middleware.utils.SyncUtils;
import org.json.JSONArray;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.time.Instant;
import java.util.*;

@Service
public class CommonJumpsellerService {
    private final CommonJumpsellerCient commonJumpsellerCient;

    @Autowired
    private ProductJumpsellerClient jumpsellerClient;

    @Autowired
    private ImageJumpsellerClient imageClient;

    @Autowired
    private CategoryRepository categoryRepository;

    @Autowired
    private ProductImageRepository imageRepository;

    @Autowired
    private IntegrationImageRepository integrationImageRepository;

    @Autowired
    private IntegrationCategoryRepository integrationCategoryRepository;

    @Autowired
    private IntegrationProductAttributesRepository integrationProductAttributesRepository;

    @Autowired
    private ProductAttributeValuesRepository productAttributeValuesRepository;

    @Autowired
    private MapperCategoryRepository mapperCategoryRepository;

    @Autowired
    private SyncProductJumpsellerRepository syncProductJumpsellerRepository;

    @Autowired
    private ProductsRepository productsRepository;

    @Autowired
    private SyncUtils syncUtils;

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

    public JumpsellerProductDto getSyncProduct(long id, String internalCode){
        JumpsellerProductDto product = jumpsellerClient.getProductById(id).block();
        if (product != null){
            Product item = updateItem(product, internalCode);
            JumpsellerImageDTO[] images = getImages(String.valueOf(id), internalCode);
            syncAttributes(id, internalCode);
        }
        System.out.println("Resync completed");

        return product;
    }

    private Product updateItem(JumpsellerProductDto product, String internalCode){
        Product item = syncItem(product.getProduct(), internalCode);

        saveItem(item, internalCode);
        return item;
    }

    private Product syncItem(Product item, String internalCode){
        MapperCategory category = mapperCategoryRepository.findByItemId(internalCode)
                .orElseGet(() -> null);

        if (category == null) {
            return item;
        }
        ProductAttributeValues name = productAttributeValuesRepository.getAttribute(internalCode, "PRODUCT_NAME")
                .orElseGet(() -> null);
        Product updateProduct = new Product();
        updateProduct.setPrice(item.getPrice());
        if (name != null){
            String productName = getName(name.getValue(), category.getBrand(), item.getSku());
            updateProduct.setName(productName);
        } else {
            updateProduct.setName(item.getName());
        }

        List<CategoryDTO> categoryList = new ArrayList<>();
        CategoryDTO rootCategoryDTO = new CategoryDTO();
        rootCategoryDTO.setId(2185804L);
        categoryList.add(rootCategoryDTO);

        CategoryDTO mainCategoryDTO = new CategoryDTO();
        mainCategoryDTO.setId(Long.valueOf(category.getCategoryId()));
        categoryList.add(mainCategoryDTO);

        if (category.getSubcategoryId() != null){
            CategoryDTO subcategoryDTO = new CategoryDTO();
            subcategoryDTO.setId(Long.valueOf(category.getSubcategoryId()));
            categoryList.add(subcategoryDTO);
        }

        if (category.getBrand() != null){
            updateProduct.setBrand(category.getBrand());
        }

        updateProduct.setCategories(categoryList.toArray(CategoryDTO[]::new));

        JumpsellerProductDto dto = new JumpsellerProductDto(updateProduct);

        JumpsellerProductDto synced = jumpsellerClient.updateProduct(item.getId(), dto).block();
        if (synced != null){
            return synced.getProduct();
        }
        System.out.println("Product: " + internalCode + " updated");
        return item;
    }

    private void saveItem(Product item, String internalCode){
        SyncJumpsellerProduct syncItem = syncProductJumpsellerRepository.findByResponseId(item.getId())
                        .orElseGet(SyncJumpsellerProduct::new);
        syncItem.setResponseId(item.getId());
        syncItem.setInternalCode(internalCode);
        syncItem.setName(item.getName());
        syncItem.setPageTitle(item.getPage_title());
        syncItem.setDescription(item.getDescription());
        syncItem.setMetaDescription(item.getMeta_description());
        syncItem.setType(item.getType());
        syncItem.setDaysToExpire(item.getDays_to_expire());
        syncItem.setPrice(item.getPrice());
        syncItem.setWeight(item.getWeight());
        syncItem.setStock(item.getStock());
        syncItem.setSku(item.getSku());
        syncItem.setBrand(item.getBrand());
        syncItem.setShippingRequired(item.isShipping_required());
        syncItem.setReviewsEnabled(item.isReviews_enabled());
        syncItem.setStatus(item.getStatus());
        syncItem.setCreatedAt(item.getCreated_at());
        syncItem.setUpdatedAt(item.getUpdated_at());
        syncItem.setPackageFormat(item.getPackage_format());
        syncItem.setLength(item.getLength());
        syncItem.setWidth(item.getWidth());
        syncItem.setHeight(item.getHeight());
        syncItem.setDiameter(item.getDiameter());
        syncItem.setPermalink(item.getPermalink());
        syncItem.setDataAreaId(DataArea.MSB.name());
        syncItem.setCompanyRefRecId(1L);

        syncProductJumpsellerRepository.save(syncItem);
        System.out.println("Product: " + internalCode + " saved");
    }

    public JumpsellerImageDTO[] getImages(String id, String internalCode){
        JumpsellerImageDTO[] syncImages = imageClient.getProductImages(id).block();
        if (syncImages != null) {
            for (JumpsellerImageDTO syncImage : syncImages ){
                Image tmpImage = syncImage.getImage();
                imageClient.deleteImage(id, tmpImage.getId().toString()).block();
            }
        }

        List<ProductImage> images = imageRepository.getImages(internalCode);
        for (ProductImage image : images){
            Image img = new Image();
            img.setUrl(image.getImageUrl());
            img.setPosition(image.getImageNumber());
            JumpsellerImageDTO imageDTO = new JumpsellerImageDTO(img);

            JumpsellerImageDTO syncDTO = imageClient.uploadImage(id, imageDTO).block();
            Products localProduct = productsRepository.getProduct(internalCode, ProductInterface.DYN.name()).orElseGet(() -> null);
            if (syncDTO != null && localProduct != null){
                Image response = syncDTO.getImage();
                IntegrationImage syncImage = integrationImageRepository.getSyncImage(internalCode, image.getImageUrl(), IntegrationType.JUMPSELLER.name())
                        .orElseGet(IntegrationImage::new);
                syncImage.setFileName(image.getBlobName());
                syncImage.setProductId(localProduct.getId());
                syncImage.setInternalCode(internalCode);
                syncImage.setProductExternalId(id);
                syncImage.setExternalId(response.getId().toString());
                syncImage.setUrl(image.getImageUrl());
                syncImage.setExternalUrl(response.getUrl());
                syncImage.setPosition(response.getPosition());
                syncImage.setDataAreaId(DataArea.MSB.name());
                syncImage.setIntegrationName(IntegrationType.JUMPSELLER.name());
                syncImage.setIntegrationParameterId(2L);
                syncImage.setCompanyId(1L);
                syncImage.setCreatedAt(Instant.now());
                syncImage.setUpdatedAt(Instant.now());

                integrationImageRepository.save(syncImage);
            }
            System.out.println("Product: " + internalCode + " images synced");
        }

        return syncImages;
    }

    public void syncAttributes(long id, String internalCode){
//        List<IntegrationProductAttribute> syncAttributes = integrationProductAttributesRepository.getSyncAttributes(internalCode, IntegrationType.JUMPSELLER.name());
        CustomField code = new CustomField();
        code.setId(76804L);
        code.setValue(internalCode);
        JumpsellerCustomFieldDTO codeDTO = new JumpsellerCustomFieldDTO(code);

        JumpsellerProductDto responseCode = jumpsellerClient.createCustomField(id, codeDTO).block();
        if (responseCode != null){
            Field[] fields = responseCode.getProduct().getFields();
            Field matchField = null;
            for (Field field : fields){
                if (Objects.equals(field.getCustomFieldId(), "76804")){
                    matchField = field;
                }
            }
            IntegrationProductAttribute syncCode = integrationProductAttributesRepository.getAttribute(internalCode, IntegrationType.JUMPSELLER.name(), "76804")
                    .orElseGet(IntegrationProductAttribute::new);
            syncCode.setAttributeId(76804L);
            syncCode.setProductId(internalCode);
            syncCode.setValue(internalCode);
            if (matchField != null) syncCode.setExternalId(matchField.getId());
            syncCode.setIntegrationName(IntegrationType.JUMPSELLER.name());

            integrationProductAttributesRepository.save(syncCode);
            System.out.println("Created code attribute " + internalCode);
        }

        ProductAttributeValues references = productAttributeValuesRepository.getAttribute(internalCode, "CROSS_REFERENCES")
                .orElseGet(() -> null);
        CustomField compatibility = new CustomField();
        compatibility.setId(76801L);
        String referenceString = "";
        if (references != null){
            referenceString = references.getValue();
        } else {
            referenceString = internalCode;
        }
        compatibility.setValue(referenceString);
        JumpsellerCustomFieldDTO compatibilityDTO = new JumpsellerCustomFieldDTO(compatibility);

        JumpsellerProductDto responseCompatibility = jumpsellerClient.createCustomField(id, compatibilityDTO).block();
        if (responseCompatibility != null){
            Field[] fields = responseCompatibility.getProduct().getFields();
            Field matchField = null;
            for (Field field : fields){
                if (Objects.equals(field.getCustomFieldId(), "76801")){
                    matchField = field;
                }
            }
            IntegrationProductAttribute syncCompatibility = integrationProductAttributesRepository.getAttribute(internalCode, IntegrationType.JUMPSELLER.name(), "76801")
                    .orElseGet(IntegrationProductAttribute::new);
            syncCompatibility.setAttributeId(76801L);
            syncCompatibility.setProductId(internalCode);
            syncCompatibility.setValue(referenceString);
            if (matchField != null) syncCompatibility.setExternalId(matchField.getId());
            syncCompatibility.setIntegrationName(IntegrationType.JUMPSELLER.name());

            integrationProductAttributesRepository.save(syncCompatibility);
            System.out.println("Created compatibility aatribute " + internalCode);
        }
    }

    private String getName(String baseName, String brand, String partNumber) {
        String name = baseName;
        boolean hasBrand = name.contains(brand);
        boolean hasPartNumber = name.contains(partNumber);
        if(!hasBrand){
            name = !Objects.equals(name, "") ? name + " "  + brand : brand;
        }
        if(!hasPartNumber){
            name = !Objects.equals(name, "") ? name + " " + partNumber : partNumber;
        }
        return syncUtils.capitalize(name);
    }

    public void updateTitles(List<ImportProductDTO> data){
        for (ImportProductDTO prod : data){
            SyncJumpsellerProduct syncItem = syncProductJumpsellerRepository.findByInternalCode(prod.getInternalCode())
                    .orElseGet(() -> null);

            if (syncItem != null){
                Product item = new Product();
                String title = prod.getTitle() + " " + prod.getSku();
                if (!Objects.equals(syncItem.getBrand(), "OTRA") || !Objects.equals(syncItem.getBrand(), "GENERICAS")){
                    title = title + " " + syncUtils.capitalize(syncItem.getBrand());
                }
                item.setName(title);
                item.setPrice(syncItem.getPrice());
                item.setPage_title(title);
//                item.setDescription(prod.getDescription());
//                item.setMeta_description(prod.getDescription());

                JumpsellerProductDto dto = new JumpsellerProductDto(item);

                JumpsellerProductDto response = jumpsellerClient.updateProduct(syncItem.getResponseId(), dto).block();
                if (response != null){
                    syncItem.setName(title);
                    syncItem.setPageTitle(title);
                    syncProductJumpsellerRepository.save(syncItem);
                    System.out.println("Updated item: " + prod.getInternalCode());
                }

                Products localItem = productsRepository.getProduct(prod.getInternalCode(), ProductInterface.DYN.name())
                        .orElseGet(() -> null);
                if (localItem != null){
                    //Save title
                    ProductAttributeValues titleAttribute = productAttributeValuesRepository.getAttribute(prod.getInternalCode(), "SEO_TITLE")
                            .orElseGet(ProductAttributeValues::new);
                    titleAttribute.setItemId(prod.getInternalCode());
                    titleAttribute.setInterfaceId(ProductInterface.DYN.name());
                    titleAttribute.setProductAttributeId("SEO_TITLE");
                    titleAttribute.setValue(title);
                    titleAttribute.setSkipNull("TRUE");
                    titleAttribute.setCreatedAt(Instant.now());
                    titleAttribute.setUpdatedAt(Instant.now());
                    titleAttribute.setDataAreaId(DataArea.MSB.name());
                    titleAttribute.setInterfaceRefRecId(4L);
                    titleAttribute.setProductAttributeRefRecId(19L);
                    titleAttribute.setCompanyRefRecId(1L);
                    titleAttribute.setProductRefRecId(localItem.getId());

                    productAttributeValuesRepository.save(titleAttribute);
                }
            }
        }
    }
}
