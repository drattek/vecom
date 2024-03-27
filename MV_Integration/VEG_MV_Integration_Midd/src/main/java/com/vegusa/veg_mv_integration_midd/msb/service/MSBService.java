package com.vegusa.veg_mv_integration_midd.msb.service;

import com.vegusa.veg_mv_integration_midd.msb.entity.EcommProducts;
import com.vegusa.veg_mv_integration_midd.msb.repository.ProductsRepository;
import com.vegusa.veg_mv_integration_midd.veg_middleware.repository.VegMvIntegrationEndptsRepository;
import jakarta.persistence.EntityManagerFactory;
import org.springframework.transaction.annotation.Transactional;
import jakarta.persistence.EntityManager;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.util.stream.Stream;


@Service
public class MSBService
{
    private final ProductsRepository productsRepository;
    private final VegMvIntegrationEndptsRepository vegMvIntegrationEndptsRepository;
    private final EntityManager entityManager;

    private int auxNumberItems = 0;

    @Autowired
    public MSBService(ProductsRepository productsRepository,
                      VegMvIntegrationEndptsRepository vegMvIntegrationEndptsRepository,
                      EntityManager entityManager)
    {
        this.productsRepository = productsRepository;
        this.vegMvIntegrationEndptsRepository = vegMvIntegrationEndptsRepository;
        this.entityManager = entityManager;
    }

    @Transactional(readOnly = true)
    public void synchronizeProducts()
    {
        Stream<EcommProducts> productsStream = productsRepository.getProducts();
        productsStream.forEach(product -> {



               // entityManager.detach(product); //error
        });

    }

    private void saveProductsWithStatus(EcommProducts ecommProducts)
    {

    }



}
