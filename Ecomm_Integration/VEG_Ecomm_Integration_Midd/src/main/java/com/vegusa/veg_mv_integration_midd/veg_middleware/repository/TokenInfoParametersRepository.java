package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.TokenInfoParameters;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface TokenInfoParametersRepository extends JpaRepository<TokenInfoParameters, Integer> {
    @Query(value = "select * from veg_ecomm_token_info_parameters vetip where vetip.integration_company = ?1",nativeQuery = true)
    TokenInfoParameters getTokenInfoParameters(String integrationCompany);
}

