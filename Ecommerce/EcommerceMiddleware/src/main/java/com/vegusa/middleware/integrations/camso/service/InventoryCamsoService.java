package com.vegusa.middleware.integrations.camso.service;

import com.vegusa.middleware.integrations.camso.client.inventory.InventoryCamsoClient;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

@Service
public class InventoryCamsoService {
    @Autowired
    private InventoryCamsoClient client;

    public String getInventory(){
        return client.getInventory();
    }
}
