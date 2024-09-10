package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SyncItemPrice;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

public interface SyncItemPriceRepository extends JpaRepository<SyncItemPrice, Long> {
    @Query(value = "select * from SynchronizedItemPrice where PriceListId = ?1 and ProductVersionId = ?2 and DataAreaId = ?3", nativeQuery = true)
    SyncItemPrice getSyncItemPrice(String priceListId, String productVersionId, String dataAreaId);
}
