package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.IntegrationImage;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

@Repository
public interface IntegrationImageRepository extends JpaRepository<IntegrationImage, Long> {
    @Query(value = "select file_name from integration_images where internal_code = :internalCode and position = :position and integration_name = 'JUMPSELLER' limit 1", nativeQuery = true)
    String getFileName(@Param("internalCode") String internalCode, @Param("position") Integer position);
}
