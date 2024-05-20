package com.vegusa.veg_ecomm_webscraperimages.repository;

import com.vegusa.veg_ecomm_webscraperimages.entity.EcommImageProducts;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface EcommImageProductsRepository extends JpaRepository <EcommImageProducts, Long>{
    @Query(value = "select max(image_number) from veg_ecomm_scraped_images vesi where vesi.internal_product_id = ?1 and vesi.veg_business_unit=?2", nativeQuery = true)
    Integer getMaxImageProductNumber(String internalProductId, String vegBusinessUnit);

}
