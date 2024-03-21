package com.vegusa.veg_mv_integration_midd.msb.controller;

import com.vegusa.veg_mv_integration_midd.msb.entity.VendTable;
import com.vegusa.veg_mv_integration_midd.msb.service.ProductsService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Controller;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;

import java.util.Collection;

@Controller
@RequestMapping("msb-mv-integration")
public class MSBController
{
    private final ProductsService productsService;

    @Autowired
    public MSBController(ProductsService productsService)
    {
        this.productsService = productsService;
    }

    @GetMapping("/upload-products")
    public void uploadProducts()
    {
        productsService.readDataLakeInfo();
    }
}
