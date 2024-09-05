package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SynchronizedItemPrice;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

public interface SyncItemPriceRepository extends JpaRepository<SynchronizedItemPrice, Long> {
    @Query(value = "select * from SynchronizedItemPrice where PriceListId = ?1 and ProductVersionId = ?2 and DataAreaId = ?3", nativeQuery = true)
    SynchronizedItemPrice getSyncItemPrice(String priceListId, String productVersionId, String dataAreaId);
}
