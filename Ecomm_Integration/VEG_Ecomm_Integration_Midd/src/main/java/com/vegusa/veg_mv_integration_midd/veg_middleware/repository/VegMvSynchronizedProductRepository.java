package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegMvSynchronizedProduct;
import jakarta.persistence.QueryHint;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.jpa.repository.QueryHints;

import java.util.stream.Stream;

import static org.hibernate.annotations.QueryHints.READ_ONLY;
import static org.hibernate.jpa.HibernateHints.HINT_CACHEABLE;
import static org.hibernate.jpa.HibernateHints.HINT_FETCH_SIZE;

public interface VegMvSynchronizedProductRepository extends JpaRepository<VegMvSynchronizedProduct, Long>
{
    @Query(value = "select * from veg_ecomm_synchronized_products vmsp where vmsp.internal_code = ?1", nativeQuery = true)
    VegMvSynchronizedProduct getSynchronizedProductById(String internalCode);

    @QueryHints(value = {
            @QueryHint(name = HINT_FETCH_SIZE, value = "" + Integer.MIN_VALUE),
            @QueryHint(name = HINT_CACHEABLE, value = "false"),
            @QueryHint(name = READ_ONLY, value = "true")
    })
    @Query(value = "select * from veg_ecomm_synchronized_products vmsp order by vmsp.internal_code", nativeQuery = true)
    Stream<VegMvSynchronizedProduct> getSynchronizedProducts();


}