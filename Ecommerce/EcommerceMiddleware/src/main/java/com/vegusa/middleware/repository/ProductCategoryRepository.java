package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.ProductCategory;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface ProductCategoryRepository  extends JpaRepository<ProductCategory, Long> {
    @Query(value = "select * from productcategory where ItemId = ?1 and DataAreaId = ?2", nativeQuery = true)
    ProductCategory getProductCategory(String itemId, String dataAreaId);

    @Query(value = "select ItemId, CategoryRefRecId from productcategory where DataAreaId = ?1 order by ItemId", nativeQuery = true)
    List<Object[]> getItemCategory(String dataAreaId);

}
