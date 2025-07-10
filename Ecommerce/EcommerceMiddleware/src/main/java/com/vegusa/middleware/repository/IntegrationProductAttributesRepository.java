package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.IntegrationProductAttribute;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface IntegrationProductAttributesRepository extends JpaRepository<IntegrationProductAttribute, Long> {
    @Query(value = "select * from integration_product_attributes where product_id = :productId and integration_name = :integrationName", nativeQuery = true)
    List<IntegrationProductAttribute> getSyncAttributes(@Param("productId") String productId, @Param("integrationName") String integrationName);

    Optional<IntegrationProductAttribute> findByExternalId(String externalId);
}
