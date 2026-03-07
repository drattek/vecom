package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.IntegrationParameter;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.Optional;

@Repository
public interface IntegrationParameterRepository extends JpaRepository<IntegrationParameter, Long> {
    Optional<IntegrationParameter> findByIntegrationName(String integrationName);

    @Query(value = "SELECT verifier FROM integration_parameters WHERE integration_name = :integrationName LIMIT 1", nativeQuery = true)
    String getVerifier(@Param("integrationName") String integrationName);
}
