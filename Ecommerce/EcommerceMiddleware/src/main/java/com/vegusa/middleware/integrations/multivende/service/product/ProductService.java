package com.vegusa.middleware.integrations.multivende.service.product;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.integrations.multivende.client.product.MultivendeProduct;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.Product;
import com.vegusa.middleware.integrations.multivende.dto.Version;
import com.vegusa.middleware.repository.*;
import com.vegusa.middleware.utils.ProductUtil;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.*;
import java.util.stream.Collectors;

@Service
public class ProductService {
    private final MultivendeProduct multivendeProduct;

    @Autowired
    private ProductAttributeValueRepository productAttributeValueRepository;

    @Autowired
    private SyncItemRepository syncItemRepository;

    @Autowired
    private ProductUtil productUtils;

    @Autowired
    private SyncBrandRepository syncBrandRepository;

    @Autowired
    private ProductCategoryRepository productCategoryRepository;

    @Autowired
    private CategoryRepository categoryRepository;

    @Autowired
    private SyncCategoryRepository syncCategoryRepository;

    @Autowired
    public ProductService(MultivendeProduct multivendeProduct) {
        this.multivendeProduct = multivendeProduct;
    }

    public Mono<EntriesDto<Product>> getAllProduct(){
        return multivendeProduct.getAllProduct();
    }

    public Mono<Void> createProduct(){

        return null;
    }

    public Mono<Void> updateProducts(Company company){
        List<String> itemIds = productAttributeValueRepository.getProdAttValueItemIds(company.getId().getDataAreaId());
        Set<String> itemIdSet = new HashSet<>(itemIds);
        System.out.println("Total items: " + itemIds.size());
        SyncItem[] syncItems = syncItemRepository.getSyncItems(itemIds, company.getId().getDataAreaId());

        List<Product> products = Arrays.stream(syncItems)
                .filter(syncItem -> itemIdSet.contains(syncItem.getInternalCode()))
                .map(syncItem -> {
                    System.out.println("Processing item: " + syncItem.getInternalCode());
                    return processUpdate(syncItem, company);
                })
                .collect(Collectors.toList());

        return Flux.fromIterable(products)
                .flatMap(product -> multivendeProduct.updateProduct(product, product.get_id()))
                .then().doOnSuccess(e -> {
                    System.out.println("Update completed");
                });
    }

    private Product processUpdate(SyncItem syncItem, Company company){
        InterfaceItems attributes = productUtils.getAttributes(syncItem.getInternalCode(), company.getId().getDataAreaId());
        SyncBrand brand = syncBrandRepository.getSyncBrand(attributes.getBrand(), company.getId().getDataAreaId());
        List<SyncCategory> categories = getCategories(syncItem.getInternalCode(), company.getId().getDataAreaId());

        return getProduct(attributes, brand, categories, syncItem);
    }

    private List<SyncCategory> getCategories(String internalCode, String dataAreaId){
        List<SyncCategory> hierarchy = new ArrayList<>();
        ProductCategory productCategory = productCategoryRepository.getProductCategory(internalCode, dataAreaId);
        if (productCategory != null){
            long currentId = productCategory.getCategory().getId().getRecId();
            Category category = categoryRepository.getCategory(currentId);

            collectCategories(category, dataAreaId, hierarchy);
        }
        return hierarchy;
    }

    private void collectCategories(Category category, String dataAreaId, List<SyncCategory> hierarchy) {
        if (category != null){
            String name = category.getId().getName();
            SyncCategory syncCategory = syncCategoryRepository.getSyncCategory(name, dataAreaId);

            if (syncCategory != null){
                hierarchy.add(syncCategory);
            }

            if (category.getParentCategory() != 0){
                Category parentCategory = categoryRepository.getCategory(category.getParentCategory());
                collectCategories(parentCategory, dataAreaId, hierarchy);
            }
        }
    }

    private Product getProduct(InterfaceItems attributes, SyncBrand brand, List<SyncCategory> categories, SyncItem syncItem){
        Product product = new Product();
        String auxShortDescription = productUtils.getShortDescription(attributes.getShortDescription(), attributes.getProductName());
        String name = productUtils.getName(attributes, attributes.getShortDescription());
        String shortDescription = productUtils.getName(attributes, auxShortDescription);
        String description = "-- TIENDA VEGUSA MAQUINARIA, DISTRUIBIDOR AUTORIZADO UNICARRIERS, BOBCAT, JLG, FLEXI. -- " +
                shortDescription + ". " + productUtils.getCrossReferences(attributes.getCrossReferences());

        product.set_id(syncItem.getResponseId());
        product.setName(name);
        product.setAlias(!attributes.getSeoTitle().isBlank() ? attributes.getSeoTitle() : name);
        product.setModel(attributes.getPartNumber());
        product.setDescription(description);
        product.setShortDescription(!attributes.getMetaDescription().isBlank() ? attributes.getMetaDescription() : shortDescription);
        product.setCode(attributes.getPartNumber());
        product.setInternalCode(attributes.getItemId());
        product.setWarrantyId("4b93c926-0e32-4681-aba4-ea1a15c89045");
        product.setInventoryTypeId("791a6654-c5f2-11e6-aad6-2c56dc130c0d");

        if (brand != null){
            product.setBrandId(brand.getResponseId());
        }

        if (!categories.isEmpty()){
            product.setProductCategoryId(categories.getFirst().getResponseId());
            if (categories.size() > 1){
                List<String> otherCategories = categories.subList(1, categories.size())
                        .stream()
                        .map(SyncCategory::getResponseId)
                        .collect(Collectors.toList());

                product.setOtherProductCategories(otherCategories.toArray(new String[0]));
            }
        }

        if (syncItem.getDefaultVersionId() != null){
            Version[] versions = new Version[] { getVersion(syncItem, attributes) };
            product.setProductVersions(versions);
        }

        String brand_name = attributes.getBrand() != null ? attributes.getBrand() : "";
        HashMap<String, String> customAttributes = new HashMap<>();
        switch (brand_name){
            case "BOBCAT":
                customAttributes.put("bea52509-9256-44b6-a2b3-7d1326c87d06", "BOBCAT");
                break;
            case "DOOSAN FK":
                customAttributes.put("bea52509-9256-44b6-a2b3-7d1326c87d06", "BOBCAT MH");
                break;
            case "CAMSO":
                customAttributes.put("bea52509-9256-44b6-a2b3-7d1326c87d06", "CAMSO");
                break;
            case "FLEXI":
            case "FELXI":
                customAttributes.put("bea52509-9256-44b6-a2b3-7d1326c87d06", "FLEXI");
                break;
            case "TVH":
            case "GENERICAS":
            case "DEKA":
            case "GEN-APYMSA":
            case "MAXILEVER":
            case "APYMSA":
            case "DONALDSON":
                customAttributes.put("bea52509-9256-44b6-a2b3-7d1326c87d06", "GENERICAS");
                break;
            case "JLG":
                customAttributes.put("bea52509-9256-44b6-a2b3-7d1326c87d06", "JLG");
                break;
            case "NISSAN":
                customAttributes.put("bea52509-9256-44b6-a2b3-7d1326c87d06", "NISSAN");
                break;
            case "RALOYD":
                customAttributes.put("bea52509-9256-44b6-a2b3-7d1326c87d06", "RAYLOD");
                break;
            case "UNICARRIERS":
                customAttributes.put("bea52509-9256-44b6-a2b3-7d1326c87d06", "UNICARRIERS");
                break;
            default:
                customAttributes.put("bea52509-9256-44b6-a2b3-7d1326c87d06", "OTRA");
                break;
        }
        if (!customAttributes.isEmpty()){
            product.setCustomAttributeValues(customAttributes);
        }

        return product;
    }

    private Version getVersion(SyncItem syncItem, InterfaceItems attributes){
        Version version = new Version();
        version.set_id(syncItem.getDefaultVersionId());

        double attribute_weight = Double.parseDouble(attributes.getWeight() == null | attributes.getWeight().isBlank() ? "0.0" : attributes.getWeight());
        if (attribute_weight > 0) attribute_weight = attribute_weight / 2.20462;
        BigDecimal weight = BigDecimal.valueOf(attribute_weight).setScale(2, RoundingMode.HALF_UP);
        version.setWeight(weight.max(new BigDecimal("1.00")));

        if (attributes.getLength() != null && !attributes.getLength().isBlank()){
            BigDecimal length = new BigDecimal(attributes.getLength()).setScale(2, RoundingMode.HALF_UP);
            version.setLength(length.max(new BigDecimal("1.00")));
        }

        if (attributes.getHeight() != null && !attributes.getHeight().isBlank()){
            BigDecimal height = new BigDecimal(attributes.getHeight()).setScale(2, RoundingMode.HALF_UP);
            version.setHeight(height.max(new BigDecimal("1.00")));
        }

        if (attributes.getWidth() != null && !attributes.getWidth().isBlank()){
            BigDecimal width = new BigDecimal(attributes.getWidth()).setScale(2, RoundingMode.HALF_UP);
            version.setWidth(width.max(new BigDecimal("1.00")));
        }
        version.setInventoryTypeId("791a6654-c5f2-11e6-aad6-2c56dc130c0d");

        return version;
    }
}
