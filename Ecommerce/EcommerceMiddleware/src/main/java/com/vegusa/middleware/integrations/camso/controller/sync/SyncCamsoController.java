package com.vegusa.middleware.integrations.camso.controller.sync;

import com.vegusa.middleware.integrations.camso.dto.AttributesCamsoDTO;
import com.vegusa.middleware.integrations.camso.service.SynchronizeCamsoService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.math.BigDecimal;
import java.util.HashMap;
import java.util.List;

@RestController
@RequestMapping("/msb-ecommerce-middleware/camso")
public class SyncCamsoController {
    @Autowired
    private SynchronizeCamsoService syncService;

    @PostMapping(value = "/create-items")
    public void createSync(@RequestBody HashMap<String, BigDecimal> request){
        BigDecimal changeType = request.get("changeType");
        syncService.CreateSync(changeType);
    }

    @PostMapping(value = "/update-sync")
    public void updateSync(@RequestBody HashMap<String, BigDecimal> request){
        BigDecimal changeType = request.get("changeType");
        syncService.UpdateSync(changeType);
    }

    @PostMapping(value = "/update-attributes")
    public void updateAttributes(@RequestBody List<AttributesCamsoDTO> request){
        syncService.UpdateCamsoProducts(request);
    }

    @PostMapping(value = "/update-images")
    public void updateImages(){
        syncService.UpdateCamsoImages();
    }
}
