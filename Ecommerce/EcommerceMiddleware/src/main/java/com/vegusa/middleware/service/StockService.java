package com.vegusa.middleware.service;

import com.vegusa.middleware.entity.SyncWarehouse;
import com.vegusa.middleware.integrations.jumpseller.dto.StockProjection;
import com.vegusa.middleware.repository.local.SyncWarehouseRepository;
import com.vegusa.middleware.utils.CommonUtils;
import com.vegusa.middleware.repository.erp.ItemInventLocationRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.math.BigDecimal;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

@Service
public class StockService {
    @Autowired
    private SyncWarehouseRepository syncWarehouseRepository;

    @Autowired
    private ItemInventLocationRepository itemInventLocationRepository;

    @Autowired
    private CommonUtils commonUtils;

    public Map<String, BigDecimal> getStocks(List<String> itemIds){
        List<List<String>> batches = commonUtils.partitionList(itemIds, 1000);

        SyncWarehouse[] syncWarehouses = syncWarehouseRepository.getSyncWarehouse();
        List<String> warehouses = new ArrayList<>();
        for (SyncWarehouse warehouse: syncWarehouses){
            warehouses.add(warehouse.getName());
        }

        Map<String, BigDecimal> stocks = new HashMap<>();
        for (List<String> batch : batches) {
            System.out.println(String.join(",", batch));
            List<StockProjection> results = itemInventLocationRepository.getStock(warehouses, batch);

            for (StockProjection result : results) {
                stocks.put(result.getArticulo(), result.getTotal());
            }
        }

        return stocks;
    }
}
