package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.IntegrationImage;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.Optional;

@Repository
public interface IntegrationImageRepository extends JpaRepository<IntegrationImage, Long> {
    @Query(value = "select file_name from integration_images where internal_code = :internalCode and position = :position and integration_name = 'JUMPSELLER' limit 1", nativeQuery = true)
    String getFileName(@Param("internalCode") String internalCode, @Param("position") Integer position);

    @Query(value = "select * from integration_images where internal_code = :itemId and url = :filename and integration_name = :integrationName", nativeQuery = true)
    Optional<IntegrationImage> getSyncImage(@Param("itemId") String itemId, @Param("filename") String filename, @Param("integrationName") String integrationName);
}
