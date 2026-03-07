package com.vegusa.middleware.controller;

import com.vegusa.middleware.dto.AttributeCompatibility;
import com.vegusa.middleware.model.erp.DYNProduct;
import com.vegusa.middleware.repository.erp.DYNProductRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/test")
public class TestController {
    @Autowired
    private DYNProductRepository dynProductRepository;

    @GetMapping("/")
    public DYNProduct[] getAll(){
        return dynProductRepository.getDYNProducts();
    }
}
