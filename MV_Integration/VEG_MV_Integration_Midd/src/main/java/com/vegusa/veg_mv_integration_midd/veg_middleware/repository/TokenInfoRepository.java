package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.TokenInfo;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface TokenInfoRepository extends JpaRepository<TokenInfo, Integer>
{
    @Query(value = "select * from veg_ecomm_token_info veti where veti.integration_company = ?1",nativeQuery = true)
    TokenInfo getTokenInfo(String integrationCompany);
}