package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VwVegEcommScrapedAdditionalInfo;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import java.util.stream.Stream;


public interface VwVegEcommScrapedAdditionalInfoRepository  extends JpaRepository<VwVegEcommScrapedAdditionalInfo, Long> {
    @Query(value = "select * from vw_veg_ecomm_scraped_additional_info vwvesai order by vwvesai.internal_product_id", nativeQuery = true)
    VwVegEcommScrapedAdditionalInfo[] getAdditionalProductsInfo();
}
