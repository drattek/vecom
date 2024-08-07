package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegEcomSynchronizedProducts;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface VegEcomSynchronizedProductsRepository extends JpaRepository<VegEcomSynchronizedProducts, Long>
{
    @Query(value = "select * from veg_ecomm_synchronized_products vmsp where vmsp.internal_code = ?1", nativeQuery = true)
    VegEcomSynchronizedProducts getSynchronizedProductById(String internalCode);
    /*
    @QueryHints(value = {
            @QueryHint(name = HINT_FETCH_SIZE, value = "" + Integer.MIN_VALUE),
            @QueryHint(name = HINT_CACHEABLE, value = "false"),
            @QueryHint(name = READ_ONLY, value = "true")
    }) */
    @Query(value = "select * from veg_ecomm_synchronized_products vmsp order by vmsp.internal_code", nativeQuery = true)
    VegEcomSynchronizedProducts[] getSynchronizedProducts();


}