package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.ProductAdditionalInfo;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

public interface ProductAdditionalInfoRepository extends JpaRepository<ProductAdditionalInfo, Long> {
    @Query(value = "select * from veg_ecomm_scraped_additional_info vesai where vesai.download_portal = ?1 and vesai.veg_business_unit = ?2 order by internal_product_id", nativeQuery = true)
    ProductAdditionalInfo[] getProductsAdditionalInfo(String interfaceId, String dataAreaId);
}
