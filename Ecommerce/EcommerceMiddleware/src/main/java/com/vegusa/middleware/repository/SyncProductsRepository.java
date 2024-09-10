package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SynchronizedProducts;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncProductsRepository extends JpaRepository<SynchronizedProducts, Long>
{
    @Query(value = "select * from veg_ecomm_synchronized_products vmsp where vmsp.internal_code = ?1", nativeQuery = true)
    SynchronizedProducts getSyncItem(String internalCode);

    @Query(value = "select * from veg_ecomm_synchronized_products vmsp order by vmsp.internal_code", nativeQuery = true)
    SynchronizedProducts[] getSyncItem();


}