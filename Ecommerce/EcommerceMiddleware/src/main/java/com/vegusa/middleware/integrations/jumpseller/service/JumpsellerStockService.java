package com.vegusa.middleware.integrations.jumpseller.service;

import com.vegusa.middleware.integrations.jumpseller.client.stock.JumpsellerStock;
import com.vegusa.middleware.integrations.jumpseller.dto.StockDto;
import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import com.vegusa.middleware.integrations.jumpseller.repository.SyncProductJumpsellerRepository;
import com.vegusa.middleware.integrations.jumpseller.utils.ProductUtils;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.util.ArrayList;
import java.util.List;

@Service
public class JumpsellerStockService {
    private final JumpsellerStock jumpsellerStock;

    @Autowired
    private SyncProductJumpsellerRepository syncProductJumpsellerRepository;

    @Autowired
    private ProductUtils productUtils;

    @Autowired
    public JumpsellerStockService(JumpsellerStock jumpsellerStock){
        this.jumpsellerStock = jumpsellerStock;
    }

    public Mono<List<StockDto>> getStock(){
        SyncJumpsellerProduct[] syncItems = syncProductJumpsellerRepository.getSyncProducts("MSB");
        List<List<String>> chunks = productUtils.getChunks(syncItems, 50);

        return Flux.fromIterable(chunks).flatMap(jumpsellerStock::getStock)
                .collectList()
                .map(listOfArrays -> {
                    List<StockDto> flatList = new ArrayList<>();
                    listOfArrays.forEach(array -> {
                        if (array != null) {
                            flatList.addAll(List.of(array));
                        }
                    });
                    return flatList;
                });
    }
}
