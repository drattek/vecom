package com.vegusa.veg_mv_integration_midd.msb.controller;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.vegusa.veg_mv_integration_midd.msb.service.MSBProductSyncService;
import com.vegusa.veg_mv_integration_midd.veg_middleware.utils.MiddUtils;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import javax.crypto.BadPaddingException;
import javax.crypto.IllegalBlockSizeException;
import javax.crypto.NoSuchPaddingException;
import java.security.InvalidAlgorithmParameterException;
import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.text.ParseException;

@RestController
@RequestMapping("msb-mv-integration")
public class MSBController {
    private final MSBProductSyncService msbProductSyncService;

    @Autowired
    public MSBController(MSBProductSyncService msbProductSyncService){ this.msbProductSyncService = msbProductSyncService; }

    @GetMapping(value = "/synchronize-products")
    public String uploadProducts(){
        try {
            msbProductSyncService.setEncryptDecryptInterface(MiddUtils.getEncryptDecryptInterface());
            msbProductSyncService.processProducts();
            return "{ \"message\" : \"" + "Product synchronization ends! " + "\" }";
        }catch ( JsonProcessingException | InvalidAlgorithmParameterException | NoSuchPaddingException | IllegalBlockSizeException |
                NoSuchAlgorithmException | BadPaddingException | InvalidKeyException | ParseException e){
            System.out.println("An error occurred while synchronizing the products.");
            System.out.println("StackTrace: ");
            e.printStackTrace();
            return "{ \"error\" : \"" + e.getStackTrace() + "\" }";
        }
    }







}
