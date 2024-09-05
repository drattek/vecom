package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SynchronizedPriceList;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncPriceListRepository extends JpaRepository<SynchronizedPriceList, Long> {
    @Query(value = "select * from synchronizedpricelist where Name = ?1 and CurrencyId = ?2 and DataAreaId = ?3", nativeQuery = true)
    SynchronizedPriceList getSyncPriceList(String name, String currencyId, String dataAreaId);

}
