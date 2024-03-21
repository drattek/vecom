package com.vegusa.veg_mv_integration_midd.msb.service;

import com.vegusa.veg_mv_integration_midd.msb.entity.VendTable;
import com.vegusa.veg_mv_integration_midd.msb.repository.ProductsRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.util.Collection;

@Service
public class ProductsService
{
    private final ProductsRepository testDataLakeRepository;

    @Autowired
    public ProductsService(ProductsRepository testDataLakeRepository)
    {
        this.testDataLakeRepository = testDataLakeRepository;
    }

    public void readDataLakeInfo()
    {
        Collection<VendTable> aux = testDataLakeRepository.test();
    }



}
