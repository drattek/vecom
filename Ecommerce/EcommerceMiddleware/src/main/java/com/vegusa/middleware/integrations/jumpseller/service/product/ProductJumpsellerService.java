package com.vegusa.middleware.integrations.jumpseller.service.product;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.constants.ProductInterface;
import com.vegusa.middleware.dto.ProductInfo;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.integrations.jumpseller.client.image.ImageJumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.client.product.ProductJumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.dto.*;
import com.vegusa.middleware.integrations.jumpseller.dto.CategoryDTO;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncProductJumpsellerRepository;
import com.vegusa.middleware.integrations.jumpseller.utils.ProductUtils;
import com.vegusa.middleware.repository.*;
import com.vegusa.middleware.service.PriceService;
import com.vegusa.middleware.service.StockService;
import com.vegusa.middleware.utils.SyncUtils;
import org.json.JSONArray;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.Instant;
import java.util.*;
import java.util.stream.Collectors;
import java.util.stream.IntStream;

@Service
public class ProductJumpsellerService {

    private final ProductJumpsellerClient jumpsellerClient;

    @Autowired
    private ProductUtils productUtils;

    @Autowired
    private SyncUtils syncUtils;

    @Autowired
    private SyncProductJumpsellerRepository syncProductJumpsellerRepository;

    @Autowired
    private SyncItemRepository syncItemRepository;

    @Autowired
    private ProductAttributeRepository productAttributeRepository;

    @Autowired
    private ProductAttributeHierarchyRepository productAttributeHierarchyRepository;

    @Autowired
    private InterfaceHierarchyRepository interfaceHierarchyRepository;

    @Autowired
    private InterfaceItemsRepository interfaceItemsRepository;

    @Autowired
    private ProductAttributeValueRepository productAttributeValueRepository;

    @Autowired
    private IntegrationCategoryRepository categoryRepository;

    @Autowired
    private ProductCategoriesRepository productCategoriesRepository;

    @Autowired
    private ProductImageRepository imageRepository;

    @Autowired
    private IntegrationAttributesRepository integrationAttributesRepository;

    @Autowired
    private IntegrationProductAttributesRepository integrationProductAttributesRepository;

    @Autowired
    private IntegrationImageRepository integrationImageRepository;

    @Autowired
    private ImageJumpsellerClient imageJumpsellerClient;

    @Autowired
    private ProductsRepository productsRepository;

    @Autowired
    private ProductAttributeValuesRepository productAttributeValuesRepository;

    @Autowired
    private StockService stockService;

    @Autowired
    private PriceService priceService;

    @Autowired
    public ProductJumpsellerService(ProductJumpsellerClient jumpsellerClient) {
        this.jumpsellerClient = jumpsellerClient;
    }

    public Mono<JumpsellerProductDto> updateProduct(long id) {
        SyncJumpsellerProduct syncItem = syncProductJumpsellerRepository.getSyncById(id);
        Product product = new Product();
        product.setName(syncItem.getName());
        product.setPrice(syncItem.getPrice());
        JumpsellerProductDto productDto = new JumpsellerProductDto(product);
        return jumpsellerClient.updateProduct(id, productDto);
    }

    public Mono<Void> deleteProduct(String id) {
        return jumpsellerClient.deleteProduct(id);
    }

    public Mono<JumpsellerProductDto> getProduct(long id) {
        return jumpsellerClient.getProductById(id);
    }

    public Mono<List<JumpsellerProductDto>> getAllProducts() {
        return jumpsellerClient.getProductCount().flatMapMany(count -> {
                    int totalPages = (int) Math.ceil(count / 100.0);
                    List<Integer> pages = IntStream.rangeClosed(1, totalPages).boxed().toList();

                    return Flux.fromIterable(pages)
                            .flatMap(jumpsellerClient::getAllProducts);
                }).collectList()
                .map(listOfArrays -> {
                    List<JumpsellerProductDto> flatList = new ArrayList<>();
                    listOfArrays.forEach(array -> {
                        if (array != null) {
                            flatList.addAll(List.of(array));
                        }
                    });
                    return flatList;
                })
                .doOnNext(products -> {
                    for (JumpsellerProductDto product : products){
                        Product jumpsellerProduct = product.getProduct();
                        SyncJumpsellerProduct syncItem = syncProductJumpsellerRepository.findByResponseId(jumpsellerProduct.getId())
                                .orElseGet(() -> new SyncJumpsellerProduct());

                        InterfaceItems localProduct = interfaceItemsRepository.getProductBySKU(jumpsellerProduct.getSku(), DataArea.MSB.name());
                        syncItem.setResponseId(jumpsellerProduct.getId());
                        syncItem.setInternalCode(localProduct.getItemId());
                        syncItem.setName(jumpsellerProduct.getName());
                        syncItem.setPageTitle(jumpsellerProduct.getPage_title());
                        syncItem.setDescription(jumpsellerProduct.getDescription());
                        syncItem.setMetaDescription(jumpsellerProduct.getMeta_description());
                        syncItem.setType(jumpsellerProduct.getType());
                        syncItem.setDaysToExpire(jumpsellerProduct.getDays_to_expire());
                        syncItem.setPrice(jumpsellerProduct.getPrice());
                        syncItem.setDiscount(jumpsellerProduct.getDiscount());
                        syncItem.setWeight(jumpsellerProduct.getWeight());
                        syncItem.setStock(jumpsellerProduct.getStock());
                        syncItem.setStockUnlimited(jumpsellerProduct.isStock_unlimited());
                        syncItem.setStockThreshold(jumpsellerProduct.getStock_threshold());
                        syncItem.setStockNotification(jumpsellerProduct.isStock_notification());
                        syncItem.setCostPerItem(jumpsellerProduct.getCost_per_item());
                        syncItem.setCompareAtPrice(jumpsellerProduct.getCompare_at_price());
                        syncItem.setMinimumQuantity(jumpsellerProduct.getMinimum_quantity());
                        syncItem.setMaximumQuantity(jumpsellerProduct.getMaximum_quantity());
                        syncItem.setSku(jumpsellerProduct.getSku());
                        syncItem.setBrand(jumpsellerProduct.getBrand());
                        syncItem.setBarcode(jumpsellerProduct.getBarcode());
                        syncItem.setGoogleProductCategory(jumpsellerProduct.getGoogle_product_category());
                        syncItem.setFeatured(jumpsellerProduct.isFeatured());
                        syncItem.setShippingRequired(jumpsellerProduct.isShipping_required());
                        syncItem.setReviewsEnabled(jumpsellerProduct.isReviews_enabled());
                        syncItem.setStatus(jumpsellerProduct.getStatus());
                        syncItem.setCreatedAt(jumpsellerProduct.getCreated_at());
                        syncItem.setUpdatedAt(jumpsellerProduct.getUpdated_at());
                        syncItem.setPackageFormat(jumpsellerProduct.getPackage_format());
                        syncItem.setLength(jumpsellerProduct.getLength());
                        syncItem.setWidth(jumpsellerProduct.getWidth());
                        syncItem.setHeight(jumpsellerProduct.getHeight());
                        syncItem.setDiameter(jumpsellerProduct.getDiameter());
                        syncItem.setPermalink(jumpsellerProduct.getPermalink());
                        syncItem.setDataAreaId(DataArea.MSB.name());
                        syncItem.setCompanyRefRecId(1L);

                        syncProductJumpsellerRepository.save(syncItem);
                        System.out.println("Updated item: " + syncItem.getInternalCode());
                    }
                })
                .doOnSuccess(products -> {
                    System.out.println("Actualización concluida");
                });
    }

    public Mono<Void> updatePrices(JSONArray pricelist, String dataAreaId){
        HashMap<String, String> itemCostMap = productUtils.getPrices(pricelist);
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts(dataAreaId);

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            //String costItem = itemCostMap.get(syncItem.getInternalCode());
            BigDecimal costItem = new BigDecimal(itemCostMap.getOrDefault(syncItem.getInternalCode(), "0.00")).setScale(2, RoundingMode.HALF_UP);
            if (costItem.compareTo(syncItem.getPrice()) != 0 && costItem.compareTo(BigDecimal.ZERO) != 0){
                Product product = new Product();
                product.setName(syncItem.getName()); // Is required
                BigDecimal price = costItem.setScale(2, RoundingMode.HALF_UP);
                //price.add(BigDecimal.valueOf(150));
                product.setPrice(price);
                JumpsellerProductDto productDto = new JumpsellerProductDto(product);

                return jumpsellerClient.updateProduct(syncItem.getResponseId(), productDto)
                        .doOnSuccess(result -> {
                            Product responseProduct = result.getProduct();
                            System.out.println("Updated product " + syncItem.getInternalCode() + " with price " + price);
                            syncItem.setPrice(responseProduct.getPrice());
                            syncProductJumpsellerRepository.save(syncItem);
                        })
                        .doOnError(error -> {
                            System.err.println("Error updating product " + syncItem.getInternalCode() + " : " + error.getMessage());
                        });
            }

            return Mono.empty();
        }).then().doOnSuccess(e -> {
            System.out.println("Pricelist Jumpseller updated ended");
        });
    }

    public Mono<Void> updateProducts(){
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts(DataArea.MSB.name());
        List<String> itemIds = new ArrayList<>();
        for (SyncJumpsellerProduct item : syncItems){
            itemIds.add(item.getInternalCode());
        }

        Map<Long, IntegrationCategory> categories = categoryRepository.findByIntegrationName(IntegrationType.JUMPSELLER.name())
                .map(list -> list.stream().collect(Collectors.toMap(
                        IntegrationCategory::getCategoryId,
                        category -> category
                )))
                .orElseGet(HashMap::new);
        IntegrationCategory mainCategory = categories.get(346L);

        Map<String, BigDecimal> prices = priceService.getPrices(itemIds);

        Map<SyncJumpsellerProduct, JumpsellerProductDto> listProducts = new HashMap<>();
        Map<String, String> crossReferences = new HashMap<>();
        System.out.println("Normalize data");
        for (SyncJumpsellerProduct syncItem : syncItems){
            Products baseProduct = syncUtils.getProductValues(syncItem.getInternalCode());
            BigDecimal basePrice = prices.getOrDefault(syncItem.getInternalCode(), BigDecimal.ZERO);
            ProductCategories baseCategory = productCategoriesRepository.getProductCategory(syncItem.getInternalCode(), DataArea.MSB.name());

            crossReferences.put(syncItem.getInternalCode(), baseProduct.getCrossReferences());

            Product product = new Product();
            BigDecimal price = basePrice.compareTo(BigDecimal.ZERO) == 0 ? syncItem.getPrice() : basePrice;
            product.setPrice(price.setScale(2, RoundingMode.HALF_UP));
            //product.setPrice(syncItem.getPrice());

            List<CategoryDTO> productCategories = new ArrayList<>();
            CategoryDTO rootCategoryDTO = new CategoryDTO();
            rootCategoryDTO.setId(Long.valueOf(mainCategory.getExternalId()));
            rootCategoryDTO.setName(mainCategory.getExternalName());
            productCategories.add(rootCategoryDTO);

            IntegrationCategory syncCategory = categories.get(baseCategory.getCategoryRefRecId());
            CategoryDTO branchCategoryDTO = new CategoryDTO();
            branchCategoryDTO.setId(Long.valueOf(syncCategory.getExternalId()));
            branchCategoryDTO.setName(syncCategory.getExternalName());
            productCategories.add(branchCategoryDTO);

            product.setCategories(productCategories.toArray(CategoryDTO[]::new));

            String name = !baseProduct.getSeoTitle().isEmpty() ? baseProduct.getSeoTitle() : syncUtils.getName(baseProduct, baseProduct.getShortDescription());
            product.setName(name);
            product.setPage_title(name);
            System.out.println(name);


            String description = syncUtils.getDescription(baseProduct, name);
            product.setDescription(description);
            product.setMeta_description(!baseProduct.getMetaDescription().isEmpty() ? baseProduct.getMetaDescription() : description);

            product.setSku(baseProduct.getPartNumber());
            product.setStatus("available");

            if (baseProduct.getWeight().compareTo(BigDecimal.ONE) > 0){
                BigDecimal weight = baseProduct.getWeight().divide(new BigDecimal("2.205"), RoundingMode.HALF_UP);
                if (weight.compareTo(BigDecimal.ONE) >= 0){
                    product.setWeight(weight.setScale(2, RoundingMode.HALF_UP));
                } else {
                    product.setWeight(BigDecimal.ONE);
                }
            } else {
                product.setWeight(BigDecimal.ONE);
            }
            if (baseProduct.getLength().compareTo(BigDecimal.ONE) > 0){
                product.setLength(baseProduct.getLength().setScale(2, RoundingMode.HALF_UP));
            } else {
                product.setLength(BigDecimal.ONE);
            }
            if (baseProduct.getHeight().compareTo(BigDecimal.ONE) > 0){
                product.setHeight(baseProduct.getHeight().setScale(2, RoundingMode.HALF_UP));
            } else {
                product.setHeight(BigDecimal.ONE);
            }
            if (baseProduct.getWidth().compareTo(BigDecimal.ONE) > 0){
                product.setWidth(baseProduct.getWidth().setScale(2, RoundingMode.HALF_UP));
            } else {
                product.setWidth(BigDecimal.ONE);
            }

            JumpsellerProductDto dto = new JumpsellerProductDto(product);

            jumpsellerClient.updateProduct(syncItem.getResponseId(), dto)
                    .doOnNext(response -> {
                        Product responseProduct = response.getProduct();
                        SyncJumpsellerProduct syncProduct = syncProductJumpsellerRepository.findByResponseId(responseProduct.getId())
                                .orElseGet(SyncJumpsellerProduct::new);
                        syncProduct.setName(responseProduct.getName());
                        syncProduct.setPageTitle(responseProduct.getPage_title());
                        syncProduct.setDescription(responseProduct.getDescription());
                        syncProduct.setMetaDescription(responseProduct.getMeta_description());
                        syncProduct.setPrice(responseProduct.getPrice());
                        syncProduct.setWeight(responseProduct.getWeight());
                        syncProduct.setHeight(responseProduct.getHeight());
                        syncProduct.setLength(responseProduct.getLength());
                        syncProduct.setWidth(responseProduct.getWidth());
                        syncProduct.setDiameter(responseProduct.getDiameter());
                        syncProduct.setUpdatedAt(responseProduct.getUpdated_at());
                        syncProduct.setPermalink(responseProduct.getPermalink());
                        syncProduct.setDataAreaId(DataArea.MSB.name());
                        syncProduct.setCompanyRefRecId(1L);

                        syncProductJumpsellerRepository.save(syncProduct);
                        System.out.println("Item " + syncItem.getInternalCode() + " successfully updated");

                        setCustomAttributes(syncItem.getInternalCode(), responseProduct).subscribe();
                    })
                    .doOnError(error -> System.err.println("Error updating " + error.getMessage()))
                    .subscribe();

            listProducts.put(syncItem, dto);
        }

        System.out.println("Update completed");

        return Mono.empty();
    }

    public Mono<Void> updateAttributes(){
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts(DataArea.MSB.name());
        Map<String, List<IntegrationProductAttribute>> currentList = integrationProductAttributesRepository
                .findByIntegrationName(IntegrationType.JUMPSELLER.name())
                .map(list -> list.stream()
                        .collect(Collectors.groupingBy(IntegrationProductAttribute::getProductId)))
                .orElse(Collections.emptyMap());

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            CustomField code = new CustomField();
            code.setId(76804L);
            code.setValue(syncItem.getInternalCode());
            JumpsellerCustomFieldDTO dto = new JumpsellerCustomFieldDTO(code);

            IntegrationProductAttribute currentAttributes = Optional.ofNullable(currentList.get(syncItem.getInternalCode()))
                    .orElse(Collections.emptyList())
                    .stream()
                    .filter(attribute -> attribute.getAttributeId() == 76804L)
                    .findFirst()
                    .orElse(null);

            if (currentAttributes == null){
                return jumpsellerClient.createCustomField(syncItem.getResponseId(), dto)
                        .doOnSuccess(result -> {
                            Product response = result.getProduct();
                            Field[] fields = response.getFields();
                            Field matchField = null;
                            for (Field field : fields){
                                if (Objects.equals(field.getCustomFieldId(), "76804")){
                                    matchField = field;
                                }
                            }
                            IntegrationProductAttribute attribute = new IntegrationProductAttribute();
                            attribute.setAttributeId(76804L);
                            attribute.setProductId(syncItem.getInternalCode());
                            attribute.setValue(syncItem.getInternalCode());
                            if (matchField != null) attribute.setExternalId(matchField.getId());
                            attribute.setIntegrationName(IntegrationType.JUMPSELLER.name());

                            integrationProductAttributesRepository.save(attribute);
                            System.out.println("Created item: " + syncItem.getInternalCode());
                        })
                        .doOnError(error -> System.err.println("Error custom field: " + error.getMessage()));
            } else {
                return jumpsellerClient.updateCustomField(syncItem.getResponseId(), currentAttributes.getExternalId(), dto)
                        .doOnSuccess(result -> {
                            CustomField field = result.getField();
                            currentAttributes.setValue(field.getValue());

                            integrationProductAttributesRepository.save(currentAttributes);
                            System.out.println("Updated item: " + syncItem.getInternalCode());
                        })
                        .doOnError(error -> System.err.println("Error update custom: " + error.getMessage()));
            }
        }).then().doOnSuccess(e -> System.out.println("Update completed"));
    }

    private Mono<Void> setCustomAttributes(String itemId, Product product){
        List<IntegrationAttributes> attributes = integrationAttributesRepository.findByIntegrationName(IntegrationType.JUMPSELLER.name())
                .orElseGet(ArrayList::new);
        List<IntegrationProductAttribute> currentList = integrationProductAttributesRepository.getSyncAttributes(itemId, IntegrationType.JUMPSELLER.name());
        Map<String, IntegrationProductAttribute> currentAttributes = new HashMap<>();
        for (IntegrationProductAttribute item : currentList){
            currentAttributes.put(item.getAttributeId().toString(), item);
        }

        Map<String, JumpsellerCustomFieldDTO> customFields = new HashMap<>();
        CustomField code = new CustomField();
        code.setId(76804L);
        code.setValue(itemId);
        JumpsellerCustomFieldDTO codeDTO = new JumpsellerCustomFieldDTO(code);
        IntegrationProductAttribute currentCode = currentAttributes.get("76804");
        if (currentCode == null){
            customFields.put("new", codeDTO);
        } else {
            customFields.put(currentCode.getExternalId(), codeDTO);
        }

        CustomField compatibility = new CustomField();
        compatibility.setId(76801L);
        ProductAttributeValues references = productAttributeValuesRepository.getAttribute(itemId, "CROSS_REFERENCES")
                .orElseGet(() -> null);
        String referenceString = "";
        if (references != null){
            referenceString = references.getValue();
        } else {
            referenceString = itemId;
        }
        System.out.println("item: " + itemId + " references: " + referenceString);
        compatibility.setValue(referenceString);
        JumpsellerCustomFieldDTO compatibilityDTO = new JumpsellerCustomFieldDTO(compatibility);
        IntegrationProductAttribute currentCompatibility = currentAttributes.get("76801");
        if (currentCompatibility == null){
            customFields.put("new", compatibilityDTO);
        } else {
            customFields.put(currentCompatibility.getExternalId(), compatibilityDTO);
        }

        return Flux.fromIterable(customFields.entrySet()).flatMap(entry -> {
            String type = entry.getKey();
            JumpsellerCustomFieldDTO item = entry.getValue();
            if (item.getField().getValue().isEmpty()){
                return Mono.empty();
            }

            if (Objects.equals(type, "new")){
                return jumpsellerClient.createCustomField(product.getId(), item)
                        .doOnSuccess(result -> {
                            Product response = result.getProduct();
                            Field[] fields = response.getFields();
                            Field matchField = null;
                            for (Field field : fields){
                                if (Objects.equals(field.getCustomFieldId(), item.getField().getId().toString())){
                                    matchField = field;
                                }
                            }
                            IntegrationProductAttribute attribute = new IntegrationProductAttribute();
                            attribute.setAttributeId(item.getField().getId());
                            attribute.setProductId(itemId);
                            attribute.setValue(item.getField().getValue());
                            if (matchField != null) attribute.setExternalId(matchField.getId());
                            attribute.setIntegrationName(IntegrationType.JUMPSELLER.name());

                            integrationProductAttributesRepository.save(attribute);
                        })
                        .doOnError(error -> System.err.println("Error custom field: " + error.getMessage()));
            } else {
                return jumpsellerClient.updateCustomField(product.getId(), type, item)
                        .doOnSuccess(result -> {
                            CustomField field = result.getField();
                            IntegrationProductAttribute attribute = integrationProductAttributesRepository.findByExternalId(type)
                                    .orElseGet(IntegrationProductAttribute::new);
                            attribute.setValue(field.getValue());

                            integrationProductAttributesRepository.save(attribute);
                        })
                        .doOnError(error -> System.err.println("Error update custom: " + error.getMessage()));
            }
        }).then().doOnSuccess(e -> {
            System.out.println("Custom field created for: " + itemId);
        });
    }

    public Mono<Void> syncProducts(String dataAreaId) {
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts(dataAreaId);

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            System.out.println("Proccesing item id: " + syncItem.getInternalCode());
            JumpsellerProductDto product = getProductData(syncItem, dataAreaId);

            return jumpsellerClient.updateProduct(syncItem.getResponseId(), product)
                    .doOnSuccess(result -> {
                        System.out.println("Updated item id: " + syncItem.getInternalCode());
                    })
                    .doOnError(error -> {
                        System.err.println("Error updating item id: " + syncItem.getInternalCode() + " - " + error.getMessage());
                    });
        }).then().doOnSuccess(e -> {
            System.out.println("Sync items ended");
        });
    }

    public Mono<Void> syncBrands(){
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts("MSB");

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            Product product = new Product();
            product.setName(syncItem.getName());
            BigDecimal price = syncItem.getPrice();
            price = price.multiply(new BigDecimal("100.00")).divide(new BigDecimal("100")).setScale(2, RoundingMode.HALF_UP); //Math.floor(price * 100) / 100.0;
            product.setPrice(price);

            String brand;
            String syncBrand = syncItem.getBrand() != null ? syncItem.getBrand() : "";
            switch (syncBrand){
                case "BOBCAT":
                    product.setBrand("BOBCAT");
                    brand = "BOBCAT";
                    break;
                case "DOOSAN FK":
                    product.setBrand("BOBCAT MH");
                    brand = "BOBCAT MH";
                    break;
                case "CAMSO":
                    product.setBrand("CAMSO");
                    brand = "CAMSO";
                    break;
                case "FLEXI":
                case "FELXI":
                    product.setBrand("FLEXI");
                    brand = "FLEXI";
                    break;
                case "TVH":
                case "GENERICAS":
                case "DEKA":
                case "GEN-APYMSA":
                case "MAXILEVER":
                case "APYMSA":
                case "DONALDSON":
                    product.setBrand("GENERICAS");
                    brand = "GENERICAS";
                    break;
                case "JLG":
                    product.setBrand("JLG");
                    brand = "JLG";
                    break;
                case "NISSAN":
                    product.setBrand("NISSAN");
                    brand = "NISSAN";
                    break;
                case "RALOYD":
                    product.setBrand("RAYLOD");
                    brand = "RAYLOD";
                    break;
                case "UNICARRIERS":
                    product.setBrand("UNICARRIERS");
                    brand = "UNICARRIERS";
                    break;
                default:
                    product.setBrand("OTRA");
                    brand = "OTRA";
                    break;
            }

            JumpsellerProductDto dto = new JumpsellerProductDto(product);

            return jumpsellerClient.updateProduct(syncItem.getResponseId(), dto)
                    .doOnSuccess(result -> {
                        System.out.println("Updated product " + syncItem.getInternalCode() + " with brand " + brand);
                        syncItem.setBrand(brand);
                        syncProductJumpsellerRepository.save(syncItem);
                    })
                    .doOnError(error -> {
                        System.err.println("Error updating product " + syncItem.getInternalCode() + " : " + error.getMessage());
                    });
        }).then().doOnSuccess(e -> {
            System.out.println("Product list updated ended");
        });
    }

    private JumpsellerProductDto getProductData (SyncJumpsellerProduct entity, String dataAreaId){
        HashMap<String, String> values = productUtils.getAttributes(entity, dataAreaId);

        JumpsellerProductDto product = productUtils.getProduct(values, entity);

        return product;
    }

    public Mono<Void> changeTitle(){
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts(DataArea.MSB.name());

        return Flux.fromArray(syncItems).flatMap(syncItem -> {
            System.out.println("Updating item: " + syncItem.getInternalCode());
            String name = capitalize(syncItem.getName());
            Product product = new Product();
            product.setName(name);
            product.setPage_title(capitalize(syncItem.getPageTitle()));
            String prefix = "-- Tienda Vegusa Maquinaria, Distribuidor autorizar Unicarriers, Bobocat, JLG, Flexi. -- ";
            product.setDescription(prefix + name);
            product.setPrice(syncItem.getPrice());

            JumpsellerProductDto dto = new JumpsellerProductDto(product);

            return jumpsellerClient.updateProduct(syncItem.getResponseId(), dto)
                    .doOnSuccess(result -> {
                        if (result != null) {
                            Product syncProduct = result.getProduct();
                            syncItem.setName(syncProduct.getName());
                            syncItem.setPageTitle(syncProduct.getPage_title());
                            syncItem.setDescription(syncProduct.getDescription());

                            syncProductJumpsellerRepository.save(syncItem);
                        }
                    })
                    .doOnError(error -> {
                        System.err.println("Error updating item: " + syncItem.getInternalCode() + " - " + error.getMessage());
                    });
        }).onErrorContinue((throwable, o) -> {
            System.err.println("Error updating");
        }).then().doOnSuccess(e -> System.out.println("Updating completed"));
    }

    private String capitalize(String str) {
        if (str == null || str.isEmpty()) {
            return str;
        }
        return str.substring(0, 1).toUpperCase() + str.substring(1).toLowerCase();
    }

    private String getBrand(String brand){
        String syncBrand = "";
        switch (brand){
            case "BOBCAT":
                syncBrand = "BOBCAT";
                break;
            case "DOOSAN FK":
                syncBrand = "BOBCAT MH";
                break;
            case "CAMSO":
                syncBrand = "CAMSO";
                break;
            case "FLEXI":
            case "FELXI":
                syncBrand = "FLEXI";
                break;
            case "TVH":
            case "GENERICAS":
            case "DEKA":
            case "GEN-APYMSA":
            case "MAXILEVER":
            case "APYMSA":
            case "DONALDSON":
                syncBrand = "GENERICAS";
                break;
            case "JLG":
                syncBrand = "JLG";
                break;
            case "NISSAN":
                syncBrand = "NISSAN";
                break;
            case "RALOYD":
                syncBrand = "RAYLOD";
                break;
            case "UNICARRIERS":
                syncBrand = "UNICARRIERS";
                break;
            default:
                syncBrand = "OTRA";
                break;
        }

        return syncBrand;
    }

    // CREATE NEW PRODUCT FROM SYNC ITEMS MULTISTORE

    public Mono<Void> syncProduct(Map<String, ProductInfo> listProducts) {
        Map<Long, IntegrationCategory> categories = categoryRepository.findByIntegrationName(IntegrationType.JUMPSELLER.name())
                .map(list -> list.stream().collect(Collectors.toMap(
                        IntegrationCategory::getCategoryId,
                        category -> category
                )))
                .orElseGet(HashMap::new);
        IntegrationCategory mainCategory = categories.get(346L);

        Map<String, JumpsellerProductDto> products = new HashMap<>();
        Map<String, JumpsellerImageDTO[]> productImages = new HashMap<>();
        for (Map.Entry<String, ProductInfo> entry : listProducts.entrySet()) {
            String itemId = entry.getKey();
            ProductInfo info = entry.getValue();
            Products baseProduct = info.getProduct();
            ProductCategories baseCategory = info.getCategory();
            List<ProductImage> images = info.getImages();

            if (syncProductJumpsellerRepository.findByInternalCode(itemId).isEmpty()){
                Product product = new Product();
                product.setPrice(info.getPrice());
                product.setStock(info.getStock().intValue());

                List<CategoryDTO> productCategories = new ArrayList<>();
                CategoryDTO rootCategoryDTO = new CategoryDTO();
                rootCategoryDTO.setId(Long.valueOf(mainCategory.getExternalId()));
                rootCategoryDTO.setName(mainCategory.getExternalName());
                productCategories.add(rootCategoryDTO);

                if (baseCategory != null){
                    IntegrationCategory syncCategory = categories.get(baseCategory.getCategoryRefRecId());
                    CategoryDTO branchCategoryDTO = new CategoryDTO();
                    branchCategoryDTO.setId(Long.valueOf(syncCategory.getExternalId()));
                    branchCategoryDTO.setName(syncCategory.getName());
                    productCategories.add(branchCategoryDTO);
                }

                product.setBrand(getBrand(baseProduct.getBrand()));

                product.setCategories(productCategories.toArray(CategoryDTO[]::new));

                String name = !baseProduct.getSeoTitle().isEmpty() ? baseProduct.getSeoTitle() : syncUtils.getName(baseProduct, baseProduct.getShortDescription());
                product.setName(name);
                product.setPage_title(name);

                String description = syncUtils.getDescription(baseProduct, name);
                product.setDescription(description);
                product.setMeta_description(!baseProduct.getMetaDescription().isEmpty() ? baseProduct.getMetaDescription() : description);

                product.setSku(baseProduct.getPartNumber());
                product.setStatus("available");

                if (baseProduct.getWeight() != null){
                    BigDecimal weight = baseProduct.getWeight().divide(new BigDecimal("2.20462"), RoundingMode.HALF_UP);
                    if (weight.compareTo(BigDecimal.ONE) >= 0){
                        product.setWeight(weight.setScale(2, RoundingMode.HALF_UP));
                    } else {
                        product.setWeight(BigDecimal.ONE);
                    }
                }
                product.setLength(baseProduct.getLength().setScale(2, RoundingMode.HALF_UP));
                product.setHeight(baseProduct.getHeight().setScale(2, RoundingMode.HALF_UP));
                product.setWidth(baseProduct.getWidth().setScale(2, RoundingMode.HALF_UP));

                JumpsellerProductDto dto = new JumpsellerProductDto(product);
                products.put(itemId, dto);

                // Images
                List<JumpsellerImageDTO> imageDTO = new ArrayList<>();
                for (ProductImage image : images){
                    Image newImage = new Image();
                    newImage.setUrl(image.getImageUrl());
                    newImage.setPosition(image.getImageNumber());
                    JumpsellerImageDTO imgDto = new JumpsellerImageDTO(newImage);

                    imageDTO.add(imgDto);
                }
                if (!imageDTO.isEmpty()){
                    productImages.put(itemId, imageDTO.toArray(JumpsellerImageDTO[]::new));
                }
            }
        }

        System.out.println("Creating product: " + products.size());
        return Flux.fromIterable(products.entrySet()).flatMap(productEntry -> {
            String itemId = productEntry.getKey();
            JumpsellerProductDto product = productEntry.getValue();
            return jumpsellerClient.createProduct(product)
                    .doOnNext(response -> {
                        Product responseProduct = response.getProduct();
                        SyncJumpsellerProduct newProduct = new SyncJumpsellerProduct();
                        newProduct.setResponseId(responseProduct.getId());
                        newProduct.setInternalCode(itemId);
                        newProduct.setName(responseProduct.getName());
                        newProduct.setPageTitle(responseProduct.getPage_title());
                        newProduct.setDescription(responseProduct.getDescription());
                        newProduct.setMetaDescription(responseProduct.getMeta_description());
                        newProduct.setType(responseProduct.getType());
                        newProduct.setDaysToExpire(responseProduct.getDays_to_expire());
                        newProduct.setPrice(responseProduct.getPrice());
                        newProduct.setDiscount(responseProduct.getDiscount());
                        newProduct.setWeight(responseProduct.getWeight());
                        newProduct.setStock(responseProduct.getStock());
                        newProduct.setStockUnlimited(responseProduct.isStock_unlimited());
                        newProduct.setStockThreshold(responseProduct.getStock_threshold());
                        newProduct.setStockNotification(responseProduct.isStock_notification());
                        newProduct.setCostPerItem(responseProduct.getCost_per_item());
                        newProduct.setCompareAtPrice(responseProduct.getCompare_at_price());
//                        newProduct.setMinimumQuantity(responseProduct.getMinimum_quantity());
//                        newProduct.setMaximumQuantity(responseProduct.getMaximum_quantity());
                        newProduct.setSku(responseProduct.getSku());
                        newProduct.setBrand(getBrand(responseProduct.getBrand()));
                        newProduct.setBarcode(responseProduct.getBarcode());
                        newProduct.setGoogleProductCategory(responseProduct.getGoogle_product_category());
                        newProduct.setFeatured(responseProduct.isFeatured());
                        newProduct.setShippingRequired(responseProduct.isShipping_required());
                        newProduct.setReviewsEnabled(responseProduct.isReviews_enabled());
                        newProduct.setStatus(responseProduct.getStatus());
                        newProduct.setCreatedAt(responseProduct.getCreated_at());
                        newProduct.setUpdatedAt(responseProduct.getUpdated_at());
                        newProduct.setPackageFormat(responseProduct.getPackage_format());
                        newProduct.setLength(responseProduct.getLength());
                        newProduct.setWidth(responseProduct.getWidth());
                        newProduct.setHeight(responseProduct.getHeight());
                        newProduct.setDiameter(responseProduct.getDiameter());
                        newProduct.setPermalink(responseProduct.getPermalink());
                        newProduct.setDataAreaId(DataArea.MSB.name());
                        newProduct.setCompanyRefRecId(1L);

                        syncProductJumpsellerRepository.save(newProduct);
                        System.out.println("Item " + itemId + " successfully created");

                        uploadImages(responseProduct.getId().toString(), itemId, productImages).subscribe();
                        setCustomAttributes(itemId, responseProduct).subscribe();
                    })
                    .doOnError(error -> System.err.println("Error creating product: " + error.getMessage()));
        }).then().doOnSuccess(e -> System.out.println("Synchronization completed"));
    }

    public Mono<String> getCustomFields(long productId){
        return jumpsellerClient.getCustomFields(productId);
    }

    private Mono<Void> uploadImages(String productId, String itemId, Map<String, JumpsellerImageDTO[]> images){
        JumpsellerImageDTO[] productImages = images.get(itemId);
        Products product = productsRepository.getProduct(itemId, ProductInterface.DYN.name()).orElseGet(Products::new);

        return Flux.fromArray(productImages).flatMap(image -> {
            return imageJumpsellerClient.uploadImage(productId, image)
                    .doOnSuccess(result -> {
                        Image response = result.getImage();
                        IntegrationImage syncImage = new IntegrationImage();
                        syncImage.setProductId(product.getId());
                        syncImage.setInternalCode(itemId);
                        syncImage.setProductExternalId(productId);
                        syncImage.setExternalId(response.getId().toString());
                        syncImage.setUrl(image.getImage().getUrl());
                        syncImage.setExternalUrl(response.getUrl());
                        syncImage.setPosition(response.getPosition());
                        syncImage.setDataAreaId(DataArea.MSB.name());
                        syncImage.setIntegrationName(IntegrationType.JUMPSELLER.name());
                        syncImage.setIntegrationParameterId(2L);
                        syncImage.setCompanyId(1L);
                        syncImage.setCreatedAt(Instant.now());
                        syncImage.setUpdatedAt(Instant.now());

                        integrationImageRepository.save(syncImage);
                    })
                    .doOnError(error -> System.err.println("Error uploading " + image.getImage().getUrl()));
        })
        .then().doOnSuccess(e -> System.out.println("Images uploaded successfully " + itemId));
    }
}

