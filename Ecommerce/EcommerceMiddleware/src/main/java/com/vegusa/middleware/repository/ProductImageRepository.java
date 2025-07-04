package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.ProductImage;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface ProductImageRepository extends JpaRepository<ProductImage, Long> {
    @Query(value = "select * from productimages where ItemId = :itemId and isActive = 1 order by Priority asc, ImageNumber asc", nativeQuery = true)
    List<ProductImage> getImages(@Param("itemId") String itemId);
}
