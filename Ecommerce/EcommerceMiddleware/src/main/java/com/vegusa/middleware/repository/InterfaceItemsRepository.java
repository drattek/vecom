package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.InterfaceItems;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface InterfaceItemsRepository extends JpaRepository<InterfaceItems, Long> {
    @Query(value = "select * from product where InterfaceId = ?1 and DataAreaId = ?2 and RecId > 19386 order by ItemId", nativeQuery = true)
    InterfaceItems[] getInterfaceProductsInfo(String interfaceId, String dataAreaId);

    @Query(value = "select * from product where ItemId = ?1 and InterfaceId = ?2 and DataAreaId = ?3", nativeQuery = true)
    InterfaceItems getInterfaceProduct(String itemId, String interfaceId, String dataAreaId);

    @Query(value = "select SkipNull from product where ItemId = ?1 and DataAreaId = ?2 limit 1", nativeQuery = true)
    String getSkipNull(String itemId, String dataAreaId);
}
