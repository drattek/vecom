package com.vegusa.veg_mv_integration_midd.msb.controller;

import com.vegusa.veg_mv_integration_midd.msb.service.MSBService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("msb-mv-integration")
public class MSBController
{
    private final MSBService msbService;

    @Autowired
    public MSBController(MSBService msbService){ this.msbService = msbService; }

    @GetMapping(value = "/synchronize-products")
    public String uploadProducts()
    {
        msbService.synchronizeProducts();

        return "It´s ok";
    }







}
