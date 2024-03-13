package com.vegusa.veg_mv_integration_midd.oauth2_0.repository;

import com.vegusa.veg_mv_integration_midd.oauth2_0.entity.TokenInfo;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository
public interface TokenInfoRepository extends JpaRepository<TokenInfo, Integer> { }