package com.vegusa.veg_mv_integration_midd.msb.controller;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.vegusa.veg_mv_integration_midd.msb.service.MSBService;
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

@RestController
@RequestMapping("msb-mv-integration")
public class MSBController
{
    private final MSBService msbService;

    @Autowired
    public MSBController(MSBService msbService){ this.msbService = msbService; }

    @GetMapping(value = "/synchronize-products")
    public String uploadProducts() throws JsonProcessingException, InvalidAlgorithmParameterException, NoSuchPaddingException,
                                        IllegalBlockSizeException, NoSuchAlgorithmException, BadPaddingException, InvalidKeyException
    {
        msbService.processProducts();

        return "It´s ok";
    }







}
