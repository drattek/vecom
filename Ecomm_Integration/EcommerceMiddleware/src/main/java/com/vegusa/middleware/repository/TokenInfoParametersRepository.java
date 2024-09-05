package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.TokenInfoParameters;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface TokenInfoParametersRepository extends JpaRepository<TokenInfoParameters, Integer> {
    @Query(value = "select * from veg_ecomm_token_info_parameters vetip where vetip.integration_company = ?1",nativeQuery = true)
    TokenInfoParameters getTokenInfoParameters(String integrationCompany);
}

