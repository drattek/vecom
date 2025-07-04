package com.vegusa.middleware.service;

import com.vegusa.middleware.dto.PriceProjection;
import com.vegusa.middleware.utils.CommonUtils;
import com.vegusa.msb.repository.ItemInventLocationRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.math.BigDecimal;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

@Service
public class PriceService {
    @Autowired
    private ItemInventLocationRepository itemInventLocationRepository;

    @Autowired
    private CommonUtils commonUtils;

    public Map<String, BigDecimal> getPrices(List<String> itemIds){
        List<List<String>> batches = commonUtils.partitionList(itemIds, 1000);

        Map<String, BigDecimal> prices = new HashMap<>();
        for (List<String> batch : batches) {
            System.out.println(String.join(",", batch));
            List<PriceProjection> results = itemInventLocationRepository.getPrices(batch);

            for (PriceProjection result : results) {
                prices.put(result.getArticulo(), result.getCost());
            }
        }

        return prices;
    }
}
