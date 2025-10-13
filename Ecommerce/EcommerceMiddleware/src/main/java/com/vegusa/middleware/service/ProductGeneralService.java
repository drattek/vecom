package com.vegusa.middleware.service;

import com.vegusa.middleware.constants.DataArea;
import com.vegusa.middleware.dto.ProductInfo;
import com.vegusa.middleware.entity.*;
import com.vegusa.middleware.integrations.jumpseller.service.product.ProductJumpsellerService;
import com.vegusa.middleware.integrations.mercadolibre.service.product.ProductMeliService;
import com.vegusa.middleware.repository.*;
import com.vegusa.middleware.utils.SyncUtils;
import org.json.JSONObject;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

@Service
public class ProductGeneralService {
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

    @Autowired
    private SyncUtils syncUtils;

    public void createProduct(List<String> itemIds){
        System.out.println("Start service");
        Map<String, BigDecimal> stocks = stockService.getStocks(itemIds);
        Map<String, BigDecimal> prices = priceService.getPrices(itemIds);

        Map<String, ProductInfo> listProducts = new HashMap<>();
        for (String itemId : itemIds){
            Boolean hasPrice = true, hasImages = true, hasMeasurement = true;
            Products product = syncUtils.getProductValues(itemId);
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
                pending.setHasWeight(product.getWeight().compareTo(BigDecimal.ZERO) != 0);

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
}
