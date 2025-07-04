package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.PriceList;
import com.vegusa.middleware.entity.PriceListId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

@Repository
public interface PriceListRepository  extends JpaRepository<PriceList, PriceListId> {
    @Query(value = "select * from PriceList where Name = ?1 and CurrencyCode = ?2 and DataAreaId = ?3", nativeQuery = true)
    PriceList getPriceList(String name, String currencyCode, String dataAreaId);

    @Query(value = "SELECT Percentage FROM pricelist WHERE Name = :name and CurrencyCode = :currency and DataAreaId = :dataAreaId", nativeQuery = true)
    String getPercentage(@Param("name") String name, @Param("currency") String currency, @Param("dataAreaId") String dataAreaId);
}
