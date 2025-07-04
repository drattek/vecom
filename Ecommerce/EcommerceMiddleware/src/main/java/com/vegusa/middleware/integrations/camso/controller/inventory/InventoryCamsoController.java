package com.vegusa.middleware.integrations.camso.controller.inventory;

import com.vegusa.middleware.integrations.camso.service.InventoryCamsoService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

@RestController
@RequestMapping("/msb-ecommerce-middleware/camso")
public class InventoryCamsoController {
    @Autowired
    private InventoryCamsoService inventoryService;

    @GetMapping(value = "/get-inventory")
    public String getInventory(){
        return inventoryService.getInventory();
    }
}
