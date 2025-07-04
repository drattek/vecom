package com.vegusa.middleware.integrations.mercadolibre.service.price;

import com.vegusa.middleware.constants.IntegrationType;
import com.vegusa.middleware.constants.PriceParameter;
import com.vegusa.middleware.entity.Category;
import com.vegusa.middleware.entity.PriceListParameter;
import com.vegusa.middleware.entity.ProductCategory;
import com.vegusa.middleware.integrations.mercadolibre.client.price.PriceMeliClient;
import com.vegusa.middleware.integrations.mercadolibre.client.product.ProductMeliClient;
import com.vegusa.middleware.integrations.mercadolibre.dto.product.ProductMeliDTO;
import com.vegusa.middleware.integrations.mercadolibre.entity.SyncItemMeli;
import com.vegusa.middleware.integrations.mercadolibre.repository.SyncItemMeliRepository;
import com.vegusa.middleware.repository.*;
import com.vegusa.msb.repository.ItemInventLocationRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.*;

@Service
public class PriceMeliService {
    @Autowired
    private PriceMeliClient client;

    @Autowired
    private ProductMeliClient productClient;

    @Autowired
    private ChannelRepository channelRepository;

    @Autowired
    private PriceListRepository priceListRepository;

    @Autowired
    private PriceListParameterRepository priceListParameterRepository;

    @Autowired
    private SyncItemMeliRepository syncItemMeliRepository;

    @Autowired
    private ItemInventLocationRepository itemInventLocationRepository;

    @Autowired
    private CategoryRepository categoryRepository;

    @Autowired
    private ProductCategoryRepository productCategoryRepository;

    public Mono<String> getPrices(String itemId){
        return client.getPrices(itemId);
    }

    public Mono<Void> updatePrices(String type, String currency, String dataAreaId){
        // Items ids
        SyncItemMeli[] items = syncItemMeliRepository.getItems(dataAreaId);
        List<String> itemIds = new ArrayList<>();
        for (SyncItemMeli item : items){
            itemIds.add(item.getInternalCode());
        }

        // Shipping cost
        PriceListParameter priceListShipping = priceListParameterRepository.getPriceListParameter(PriceParameter.SHIPPING_COST.name(), dataAreaId);
        BigDecimal shippingCost = priceListShipping.getDecValue();

        // Channel base percentage
        String base_percentage = priceListRepository.getPercentage(type, currency, dataAreaId);
        BigDecimal percentage_base = new BigDecimal(base_percentage);
        String channel_percentage = channelRepository.getPercentage(IntegrationType.MERCADO_LIBRE.name(), currency, dataAreaId);
        BigDecimal percentage_channel = new BigDecimal(channel_percentage);
        BigDecimal total_percentage = percentage_base.add(percentage_channel);

        HashMap<String, String> itemCostMap = getItemMap(itemInventLocationRepository.getCosts(itemIds));
        Category[] categories = categoryRepository.getAllCategories(dataAreaId);

        Map<String, ProductMeliDTO> products = new HashMap<>();
        for (SyncItemMeli item : items){
            if (itemCostMap.get(item.getInternalCode()) != null && item.getStatus().equals("active")){
                ProductCategory productCategory = productCategoryRepository.getProductCategory(item.getInternalCode(), dataAreaId);
                Optional<Category> currentCategory = Arrays.stream(categories)
                        .filter(category -> Objects.equals(category.getId().getRecId(), productCategory.getCategory().getId().getRecId()))
                        .findFirst();

                BigDecimal categoryPercentage = currentCategory.isPresent() ? currentCategory.get().getPercentage() : new BigDecimal("35");

                BigDecimal productCost = new BigDecimal(itemCostMap.get(item.getInternalCode()));
                BigDecimal auxCost = total_percentage
                        .add(categoryPercentage)
                        .divide(new BigDecimal("100"), 2, RoundingMode.HALF_UP)
                        .add(BigDecimal.ONE)
                        .multiply(productCost);
                BigDecimal finalCost = getAdditionalFixedCost(auxCost, dataAreaId).add(shippingCost).setScale(2, RoundingMode.HALF_UP);

                if (item.getPrice().compareTo(finalCost) != 0){
                    ProductMeliDTO auxProduct = new ProductMeliDTO();
                    auxProduct.setPrice(finalCost);
                    products.put(item.getResponseId(), auxProduct);
                }
                //System.out.println("Item " + item.getInternalCode() + " with price: " + finalCost);
            }
        }

        if (!products.isEmpty()){
            return Flux.fromIterable(products.entrySet()).flatMap(item -> {
                String meliId = item.getKey();
                ProductMeliDTO product = item.getValue();

                return productClient.updateProduct(product, meliId)
                        .doOnSuccess(result -> {
                            System.out.println("Updated product " + result.getId() + "with price " + product.getPrice());
                            Optional<SyncItemMeli> filterItem = Arrays.stream(items)
                                    .filter(syncItem -> meliId.equals(syncItem.getResponseId()))
                                    .findFirst();

                            if (filterItem.isPresent()){
                                SyncItemMeli syncItem = filterItem.get();
                                syncItem.setPrice(result.getPrice());
                                syncItem.setStatus(result.getStatus());
                                syncItemMeliRepository.save(syncItem);
                            }
                        })
                        .doOnError(error -> {
                            System.err.println("Error: " + error.getMessage());
                        });
            }).then().doOnSuccess(e -> {
                System.out.println("Prices mercado libre update completed");
            });
        }
        System.out.println("There are not prices to update in Mercado Libre");
        return Mono.empty();
    }

    private HashMap<String, String> getItemMap(List<Object[]> itemValues) throws RuntimeException {
        HashMap<String, String> response = new HashMap<>();
        for(Object[] itemValue: itemValues){
            try {
                if(itemValue[0] != null && itemValue[1] != null) {
                    response.put(itemValue[0].toString(), itemValue[1].toString());
                }
            }catch (RuntimeException e){
                System.err.println("An error occurred while saving item value map.");
            }
        }
        return response;
    }

    private BigDecimal getAdditionalFixedCost(BigDecimal cost, String dataAreaId){
        PriceListParameter lowerLimit = priceListParameterRepository.getPriceListParameter(PriceParameter.ADDITIONAL_FIXED_COST_LL.name(), dataAreaId);
        PriceListParameter centralLimit = priceListParameterRepository.getPriceListParameter(PriceParameter.ADDITIONAL_FIXED_COST_CL.name(), dataAreaId);
        PriceListParameter upperLimit = priceListParameterRepository.getPriceListParameter(PriceParameter.ADDITIONAL_FIXED_COST_UL.name(), dataAreaId);

        if (lowerLimit != null && centralLimit != null && upperLimit != null){
            if (cost.compareTo(new BigDecimal(lowerLimit.getIntValue())) < 0){
                return new BigDecimal(lowerLimit.getIntValue())
                        .add(lowerLimit.getDecValue());
            } else if (cost.compareTo(new BigDecimal(centralLimit.getIntValue())) < 0){
                return cost
                        .add(lowerLimit.getDecValue());
            } else if (cost.compareTo(new BigDecimal(upperLimit.getIntValue())) < 0){
                return cost
                        .add(centralLimit.getDecValue());
            }
            return cost.add(upperLimit.getDecValue());
        }
        return new BigDecimal("0.00");
    }
}
