package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SyncWarehouse;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncWarehouseRepository extends JpaRepository<SyncWarehouse, Long> {
    @Query(value = "select * from SyncWarehouse order by Name", nativeQuery = true)
    SyncWarehouse[] getSyncWarehouse();

    @Query(value = "select * from SyncWarehouse where Name = ?1", nativeQuery = true)
    SyncWarehouse getSyncWarehouse(String name);
}
