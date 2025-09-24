package com.vegusa.middleware.integrations.camso.service;

import com.vegusa.middleware.integrations.camso.client.inventory.InventoryCamsoClient;
import com.vegusa.middleware.integrations.camso.dto.InventoryCamsoDTO;
import com.vegusa.middleware.integrations.camso.dto.ItemCamsoDTO;
import com.vegusa.middleware.integrations.camso.entity.ProductCamso;
import com.vegusa.middleware.integrations.camso.repository.ProductCamsoRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.Objects;

@Service
public class InventoryCamsoService {
    @Autowired
    private InventoryCamsoClient client;

    @Autowired
    private ProductCamsoRepository productCamsoRepository;

    public InventoryCamsoDTO getInventory(){
        InventoryCamsoDTO response = client.getInventory();
        Map<String, List<ItemCamsoDTO>> inventory = response.getItems();

        for (Map.Entry<String, List<ItemCamsoDTO>> entry : inventory.entrySet()){
            List<ItemCamsoDTO> products = entry.getValue();

            for (ItemCamsoDTO product : products){
                ProductCamso syncItem = productCamsoRepository.findByPartNumber(product.getItemCode())
                        .orElseGet(() -> {
                            ProductCamso tmp = new ProductCamso();
                            tmp.setPartNumber(product.getItemCode());
                            tmp.setName(product.getItemName());
                            tmp.setBrand(product.getBrand());
                            tmp.setCategory(product.getClasgral());
                            tmp.setCurrency(product.getCurrency());
                            tmp.setProductGroup(product.getGroup());
                            tmp.setSync(false);
                            return tmp;
                        });
                if (product.getPrice() == null){
                    syncItem.setPrice(product.getCurrency() + " 0.00");
                } else {
                    syncItem.setPrice(product.getPrice());
                }

                BigDecimal stock = BigDecimal.ZERO;
                Map<String, BigDecimal> locations = product.getLocations();
                for (Map.Entry<String, BigDecimal> location : locations.entrySet()){
                    stock = stock.add(location.getValue());
                }
                syncItem.setStock(stock.setScale(2, RoundingMode.HALF_UP));

//                syncItem.setSync(false);
                syncItem.setUpdatedAt(Instant.now());

                productCamsoRepository.save(syncItem);
            }
        }

        List<ProductCamso> outdated = productCamsoRepository.getOutdateProducts();

        for (ProductCamso outdatedItem : outdated){
            outdatedItem.setStock(BigDecimal.ZERO);
            outdatedItem.setUpdatedAt(Instant.now());
            productCamsoRepository.save(outdatedItem);
        }

        return response;
    }
}
