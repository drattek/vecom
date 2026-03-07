package com.vegusa.middleware.service.erp;

import com.vegusa.middleware.model.erp.EcomProduct;
import com.vegusa.middleware.repository.erp.EcomProductRepository;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.util.List;

@Service
public class EcomProductService {
    @Autowired
    private EcomProductRepository ecomProductRepository;

    public List<EcomProduct> getAll(){
        return ecomProductRepository.findAll();
    }
}
