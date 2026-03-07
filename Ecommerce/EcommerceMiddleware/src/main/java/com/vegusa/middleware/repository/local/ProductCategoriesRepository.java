package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.ProductCategories;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface ProductCategoriesRepository extends JpaRepository<ProductCategories, Long> {
    @Query(value = "select * from productcategory where ItemId = ?1 and DataAreaId = ?2", nativeQuery = true)
    ProductCategories getProductCategory(String itemId, String dataAreaId);
}
