package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.Category;
import com.vegusa.middleware.entity.CategoryId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface CategoryRepository extends JpaRepository<Category, CategoryId> {
    @Query(value = "select * from Category where Name = ?1 and CurrencyCode = ?2 and DataAreaId = ?3", nativeQuery = true)
    Category getCategory(String name, String currencyCode, String dataAreaId);
}
