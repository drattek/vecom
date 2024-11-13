package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SyncCategory;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncCategoryRepository extends JpaRepository<SyncCategory, Long> {
    @Query(value = "select * from SyncCategory where Name = ?1 and DataAreaId = ?2", nativeQuery = true)
    SyncCategory getSyncCategory(String categoryName, String dataAreaId);
}
