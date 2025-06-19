package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.IntegrationParameter;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.Optional;

@Repository
public interface IntegrationParameterRepository extends JpaRepository<IntegrationParameter, Long> {
    Optional<IntegrationParameter> findByIntegrationName(String integrationName);
}
