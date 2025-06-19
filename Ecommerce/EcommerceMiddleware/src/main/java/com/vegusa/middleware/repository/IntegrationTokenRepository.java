package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.IntegrationToken;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.Optional;

@Repository
public interface IntegrationTokenRepository extends JpaRepository<IntegrationToken, Long> {
    Optional<IntegrationToken> findByIntegrationName(String integrationNAme);
}
