package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.InterfaceProduct;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface InterfaceProductRepository extends JpaRepository<InterfaceProduct, Long> {
    @Query(value = "select * from product p where p.InterfaceId = ?1 and p.DataAreaId = ?2 order by p.ItemId", nativeQuery = true)
    InterfaceProduct[] getInterfaceProductsInfo(String interfaceId, String dataAreaId);

    @Query(value = "select * from product p where p.ItemId = ?1 and p.InterfaceId = ?2 and p.DataAreaId = ?3", nativeQuery = true)
    InterfaceProduct getInterfaceProduct(String itemId, String interfaceId, String dataAreaId);

    @Query(value = "select SkipNull from product p where p.ItemId = ?1 and p.DataAreaId = ?2 limit 1", nativeQuery = true)
    String getSkipNull(String itemId, String dataAreaId);
}
