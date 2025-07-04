package com.vegusa.middleware.service;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.dto.ProductInfo;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.integrations.jumpseller.service.product.ProductJumpsellerService;
import com.vegusa.middleware.integrations.mercadolibre.service.product.ProductMeliService;
import com.vegusa.middleware.repository.*;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.Instant;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

@Service
public class ProductGeneralService {
    @Autowired
    private ProductAttributeRepository attributeRepository;

    @Autowired
    private ProductAttributeValueRepository attributeValueRepository;

    @Autowired
    private ProductAttributeHierarchyRepository hierarchyRepository;

    @Autowired
    private InterfaceHierarchyRepository interfaceHierarchyRepository;

    @Autowired
    private ProductsRepository productsRepository;

    @Autowired
    private ProductCategoriesRepository categoryRepository;

    @Autowired
    private ProductImageRepository imageRepository;

    @Autowired
    private ProductPendingsRepository pendingsRepository;

    @Autowired
    private StockService stockService;

    @Autowired
    private PriceService priceService;

    @Autowired
    private ProductJumpsellerService jumpsellerService;

    @Autowired
    private ProductMeliService meliService;

    public void createProduct(List<String> itemIds){
        System.out.println("Start service");
        Map<String, BigDecimal> stocks = stockService.getStocks(itemIds);
        Map<String, BigDecimal> prices = priceService.getPrices(itemIds);

        Map<String, ProductInfo> listProducts = new HashMap<>();
        for (String itemId : itemIds){
            Products product = getProductValues(itemId);
            ProductCategories category = categoryRepository.getProductCategory(itemId, DataArea.MSB.name());
            BigDecimal stock = stocks.getOrDefault(itemId, BigDecimal.ZERO);
            BigDecimal price = prices.getOrDefault(itemId, BigDecimal.ZERO);
            List<ProductImage> images = imageRepository.getImages(itemId);

            if (price.compareTo(BigDecimal.ZERO) == 0 || images.isEmpty() || product.getWeight().compareTo(BigDecimal.ZERO) == 0){
                ProductPendings pending = pendingsRepository.findByInternalCode(itemId)
                        .orElseGet(ProductPendings::new);
                pending.setInternalCode(itemId);
                pending.setHasPrice(price.compareTo(BigDecimal.ZERO) != 0);
                pending.setHasImage(!images.isEmpty());
                pending.setHasWeight(product.getWeight().compareTo(BigDecimal.ZERO) == 0);

                pendingsRepository.save(pending);
                continue;
            }

            ProductInfo info = new ProductInfo();
            info.setProduct(product);
            info.setCategory(category);
            info.setStock(stock);
            info.setPrice(price.setScale(2, RoundingMode.HALF_UP));
            info.setImages(images);
            listProducts.put(itemId, info);

            pendingsRepository.findByInternalCode(itemId).ifPresent(pendingsRepository::delete);
        }

        System.out.println("Jumpseller starting");
        jumpsellerService.syncProduct(listProducts).subscribe();

        System.out.println("Mercado Libre starting");
        meliService.syncProduct(listProducts).subscribe();
    }

    private Products getProductValues(String internalCode){
        List<String> attributes = attributeRepository.getProductAttributeId(DataArea.MSB.name());
        HashMap<String, String> attributeMap = new HashMap<>();
        List<String> auxHierarchy;
        String auxAttributeValue;
        for (String attribute : attributes){
            auxHierarchy = hierarchyRepository.getAttributeHierarchy(attribute, DataArea.MSB.name());
            if (auxHierarchy.isEmpty()){
                auxHierarchy = interfaceHierarchyRepository.getInterfaceHierarchy(DataArea.MSB.name());
            }

            auxAttributeValue = getAttributeValue(auxHierarchy, attribute, internalCode);
            attributeMap.put(attribute, auxAttributeValue);
        }

        return getProductBase(attributeMap);
    }

    private String getAttributeValue(List<String> hierarchies, String attribute, String itemId){
        boolean skipNull = Boolean.parseBoolean(productsRepository.getSkipNull(itemId, DataArea.MSB.name()));
        String response = "";

        for (String hierarchy : hierarchies) {
            ProductAttributeValue auxAttributeValue = attributeValueRepository.getProductAttributeValue(attribute, itemId, hierarchy, DataArea.MSB.name());
            if (auxAttributeValue != null) {
                response = auxAttributeValue.getValue();
                break;
            }
            if (!skipNull) {
                break;
            }
        }

        return response;
    }

    private Products getProductBase(HashMap<String, String> values) {
        Products product = new Products();
        product.setItemId(values.get("ITEM_ID"));
        product.setProductName(values.get("PRODUCT_NAME"));
        product.setPartNumber(values.get("PART_NUMBER"));
        product.setShortDescription(values.get("SHORT_DESCRIPTION"));
        product.setBrand(values.get("BRAND_ID"));
        product.setCategory(values.get("CATEGORY_ID"));
        BigDecimal weight = values.get("WEIGHT") != null && !values.get("WEIGHT").isBlank() ? new BigDecimal(values.get("WEIGHT")) : BigDecimal.ZERO;
        product.setWeight(weight);
        product.setUnitOfMeasurement(values.get("UNIT_OF_MEASUREMENT"));
        if(values.get("AVAILABLE") != null && !values.get("AVAILABLE").isBlank()){
            product.setAvailable(new BigDecimal(values.get("AVAILABLE")).longValue());
        }
        if(values.get("COST") != null || !values.get("COST").isBlank()){
            product.setCost(new BigDecimal(values.getOrDefault("COST", "0.00")));
        }
        BigDecimal length = values.get("LENGTH") != null && !values.get("LENGTH").isBlank() ? new BigDecimal(values.get("LENGTH")) : BigDecimal.ZERO;
        product.setLength(length);
        BigDecimal height = values.get("HEIGHT") != null && !values.get("HEIGHT").isBlank() ? new BigDecimal(values.get("HEIGHT")) : BigDecimal.ZERO;
        product.setHeight(height);
        BigDecimal width = values.get("WIDTH") != null && !values.get("WIDTH").isBlank() ? new BigDecimal(values.get("WIDTH")) : BigDecimal.ZERO;
        product.setWidth(width);
        product.setSeoTitle(values.get("SEO_TITLE"));
        product.setMetaDescription(values.get("META_DESCRIPTION"));
        product.setCrossReferences(values.get("CROSS_REFERENCES"));
        product.setUpdatedAt(Instant.now());

        return product;
    }

}
