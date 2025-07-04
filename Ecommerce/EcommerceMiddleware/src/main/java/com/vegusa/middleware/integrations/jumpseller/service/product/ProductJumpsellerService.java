package com.vegusa.middleware.integrations.jumpseller.service.product;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.dto.ProductInfo;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.integrations.jumpseller.client.product.ProductJumpsellerClient;
import com.vegusa.middleware.integrations.jumpseller.dto.Category;
import com.vegusa.middleware.integrations.jumpseller.dto.JumpsellerProductDto;
import com.vegusa.middleware.integrations.jumpseller.dto.Product;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncProductJumpsellerRepository;
import com.vegusa.middleware.integrations.jumpseller.utils.ProductUtils;
import com.vegusa.middleware.repository.*;
import com.vegusa.middleware.utils.SyncUtils;
import org.json.JSONArray;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
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
                BigDecimal final_price = price; //price.add(BigDecimal.valueOf(150));
                product.setPrice(final_price);
                JumpsellerProductDto productDto = new JumpsellerProductDto(product);

                return jumpsellerClient.updateProduct(syncItem.getResponseId(), productDto)
                        .doOnSuccess(result -> {
                            System.out.println("Updated product " + syncItem.getInternalCode() + " with price " + final_price);
                            syncItem.setPrice(final_price);
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

    // CREATE NEW PRODUCT FROM SYNC ITEMS MULTISTORE

    public Mono<Void> syncProduct(Map<String, ProductInfo> listProducts) {
        Map<Long, IntegrationCategory> categories = categoryRepository.findByIntegrationName(IntegrationType.JUMPSELLER.name())
                .map(list -> list.stream().collect(Collectors.toMap(
                        IntegrationCategory::getCategoryId,
                        category -> category
                )))
                .orElseGet(HashMap::new);
        IntegrationCategory mainCategory = categories.get(346L);

        List<JumpsellerProductDto> products = new ArrayList<>();
        for (Map.Entry<String, ProductInfo> entry : listProducts.entrySet()) {
            String itemId = entry.getKey();
            ProductInfo info = entry.getValue();
            Products baseProduct = info.getProduct();
            ProductCategories baseCategory = info.getCategory();

            if (syncProductJumpsellerRepository.findByInternalCode(itemId).isEmpty()){
                Product product = new Product();
                product.setPrice(info.getPrice());
                product.setStock(info.getStock().intValue());

                List<Category> productCategories = new ArrayList<>();
                Category rootCategory = new Category();
                rootCategory.setId(Long.valueOf(mainCategory.getExternalId()));
                rootCategory.setName(mainCategory.getExternalName());
                productCategories.add(rootCategory);

                IntegrationCategory syncCategory = categories.get(baseCategory.getCategoryRefRecId());
                Category branchCategory = new Category();
                branchCategory.setId(Long.valueOf(syncCategory.getExternalId()));
                branchCategory.setName(syncCategory.getName());
                productCategories.add(branchCategory);

                product.setCategories(productCategories.toArray(Category[]::new));

                String name = !baseProduct.getSeoTitle().isEmpty() ? baseProduct.getSeoTitle() : syncUtils.getName(baseProduct, baseProduct.getShortDescription());
                product.setName(name);
                product.setPage_title(name);
                System.out.println(name);

                String description = syncUtils.getDescription(baseProduct, name);
                product.setDescription(description);
                System.out.println(description);

                product.setSku(baseProduct.getPartNumber());
                product.setStatus("available");

                product.setWeight(baseProduct.getWeight());
                product.setLength(baseProduct.getLength());
                product.setHeight(baseProduct.getHeight());
                product.setWidth(baseProduct.getWidth());

                JumpsellerProductDto dto = new JumpsellerProductDto(product);
                products.add(dto);
            }
        }

        return Flux.fromArray(products.toArray(JumpsellerProductDto[]::new)).flatMap(product ->
                jumpsellerClient.createProduct(product)
                    .doOnSuccess(response -> {
                        System.out.println("Product created: " + product.getProduct().getSku());
                        SyncJumpsellerProduct newProduct = new SyncJumpsellerProduct();
                    })
                    .doOnError(error -> System.err.println("Error creating product: " + error.getMessage()))
        ).then().doOnSuccess(e -> System.out.println("Synchronization completed"));
    }
}

