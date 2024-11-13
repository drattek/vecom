package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SyncBrand;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncBrandRepository extends JpaRepository<SyncBrand, Long> {
    @Query(value = "select * from SyncBrand where Name = ?1 and DataAreaId = ?2", nativeQuery = true)
    SyncBrand getSyncBrand(String brandName, String dataAreaId);
}
