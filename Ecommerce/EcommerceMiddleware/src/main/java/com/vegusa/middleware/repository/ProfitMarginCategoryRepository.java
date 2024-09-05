package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.ProfitMarginCategory;
import com.vegusa.middleware.entity.ProfitMarginCategoryId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface ProfitMarginCategoryRepository extends JpaRepository<ProfitMarginCategory, ProfitMarginCategoryId> {
    @Query(value = "select * from ProfitMarginCategory where CheckedByItem = ?1 and IsActive = ?2", nativeQuery = true)
    ProfitMarginCategory[] getProfitMarginCategory(boolean checkedByItem, boolean isActive);
}
