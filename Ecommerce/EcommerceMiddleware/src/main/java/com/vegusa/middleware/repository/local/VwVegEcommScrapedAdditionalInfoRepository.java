package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.VwVegEcommScrapedAdditionalInfo;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;


@Repository
public interface VwVegEcommScrapedAdditionalInfoRepository  extends JpaRepository<VwVegEcommScrapedAdditionalInfo, Long> {
    @Query(value = "select * from vw_veg_ecomm_scraped_additional_info vwvesai order by vwvesai.internal_product_id", nativeQuery = true)
    VwVegEcommScrapedAdditionalInfo[] getAdditionalProductsInfo();

    @Query(value = "select * from vw_veg_ecomm_scraped_additional_info vwvesai where vwvesai.download_portal = ?1 order by vwvesai.internal_product_id", nativeQuery = true)
    VwVegEcommScrapedAdditionalInfo[] getAdditionalProductsInfoByInterface(String interfaceName);
}
