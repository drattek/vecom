package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegEcommScrapedImage;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegMvSynchronizedProduct;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

public interface VegEcommScrapedImageRepository  extends JpaRepository<VegEcommScrapedImage, Long> {
    @Query(value = "select * from veg_ecomm_scraped_images vesi where vesi.internal_product_id = ?1", nativeQuery = true)
    VegEcommScrapedImage[] getImagesOfProduct(String internalProductId);
}
