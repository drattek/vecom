package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.ProductImagesView;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface ProductImagesViewRepository extends JpaRepository<ProductImagesView, Long> {
    @Query(value = "select * from productimagesview where DataAreaId = ?1 and isActive = 1 order by ItemId, Priority Asc", nativeQuery = true)
    ProductImagesView[] getProductImagesView(String dataAreaId);
}
