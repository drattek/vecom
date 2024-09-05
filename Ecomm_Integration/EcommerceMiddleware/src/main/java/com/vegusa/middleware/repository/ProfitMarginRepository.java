package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.ProfitMargin;
import com.vegusa.middleware.entity.ProfitMarginId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface ProfitMarginRepository extends JpaRepository<ProfitMargin, ProfitMarginId> {
    @Query(value = "select * from ProfitMargin where CategoryName = ?1 and Name = ?2 and CurrencyCode = ?3 and DataAreaId = ?4", nativeQuery = true)
    ProfitMargin getProfitMargin(String categoryName, String name, String currencyCode, String dataAreaId);
}
