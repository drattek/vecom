package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.VegEcommScrapedImage;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface VegEcommScrapedImageRepository  extends JpaRepository<VegEcommScrapedImage, Long> {
   @Query(value = "select * from veg_ecomm_scraped_images vesi where vesi.internal_product_id = ?1", nativeQuery = true)
   VegEcommScrapedImage[] getImagesOfProduct(String internalProductId);
}
