package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.IntegrationAttributes;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface IntegrationAttributesRepository extends JpaRepository<IntegrationAttributes, Long> {
    Optional<List<IntegrationAttributes>> findByIntegrationName(String integrationName);
}
