package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SyncPriceList;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncPriceListRepository extends JpaRepository<SyncPriceList, Long> {
    @Query(value = "select * from SyncPriceList where Name = ?1 and CurrencyId = ?2 and DataAreaId = ?3", nativeQuery = true)
    SyncPriceList getSyncPriceList(String name, String currencyId, String dataAreaId);

}
