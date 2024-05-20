package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegMvSynchronizedProduct;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

import java.util.stream.Stream;

public interface VegMvSynchronizedProductRepository extends JpaRepository<VegMvSynchronizedProduct, Long>
{
    @Query(value = "select * from veg_ecomm_synchronized_products vmsp where vmsp.internal_code = ?1", nativeQuery = true)
    VegMvSynchronizedProduct getSynchronizedProductById(String internalCode);

    @Query(value = "select * from veg_ecomm_synchronized_products vmsp order by vmsp.internal_code", nativeQuery = true)
    Stream<VegMvSynchronizedProduct> getSynchronizedProducts();


}