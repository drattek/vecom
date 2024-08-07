package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VwVegEcommScrapedAdditionalInfo;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.stream.Stream;


@Repository
public interface VwVegEcommScrapedAdditionalInfoRepository  extends JpaRepository<VwVegEcommScrapedAdditionalInfo, Long> {
    @Query(value = "select * from vw_veg_ecomm_scraped_additional_info vwvesai order by vwvesai.internal_product_id", nativeQuery = true)
    VwVegEcommScrapedAdditionalInfo[] getAdditionalProductsInfo();

    @Query(value = "select * from vw_veg_ecomm_scraped_additional_info vwvesai where vwvesai.download_portal = ?1 order by vwvesai.internal_product_id", nativeQuery = true)
    VwVegEcommScrapedAdditionalInfo[] getAdditionalProductsInfoByInterface(String interfaceName);
}
