package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.IntegrationCategory;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.util.List;
import java.util.Optional;

public interface IntegrationCategoryRepository extends JpaRepository<IntegrationCategory, Long> {
    @Query(value = "select * from integration_category where CategoryId = :categoryId and DataAreaId = :dataAreaId and IntegrationName = :integrationName", nativeQuery = true)
    IntegrationCategory getCategory(@Param("categoryId") long categoryId, @Param("dataAreaId") String dataAreaId, @Param("integrationName") String integrationName);

    @Query(value = "select * from integration_category where ExternalId = :externalId and DataAreaId = :dataAreaId", nativeQuery = true)
    IntegrationCategory getCategoryByExternal(@Param("externalId") String externalId, @Param("dataAreaId") String dataAreaId);

    @Query(value = "select * from integration_category where ExternalId = :externalId and DataAreaId = :dataAreaId and IntegrationName = :integrationName", nativeQuery = true)
    List<IntegrationCategory> getCategories(@Param("externalId") String externalId, @Param("dataAreaId") String dataAreaId, @Param("integrationName") String integrationName);

    Optional<List<IntegrationCategory>> findByIntegrationName(String integrationName);
}
