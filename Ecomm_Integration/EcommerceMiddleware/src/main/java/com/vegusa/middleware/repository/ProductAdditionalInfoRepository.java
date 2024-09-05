package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.ProductAdditionalInfo;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

public interface ProductAdditionalInfoRepository extends JpaRepository<ProductAdditionalInfo, Long> {
    @Query(value = "select * from itemscrapedinformation isi where isi.download_portal = ?1 and isi.veg_business_unit = ?2 order by isi.internal_product_id", nativeQuery = true)
    ProductAdditionalInfo[] getProductsAdditionalInfo(String interfaceId, String dataAreaId);
}
