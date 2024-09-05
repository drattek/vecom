package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SynchronizedWarehouse;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncWarehouseRepository extends JpaRepository<SynchronizedWarehouse, Long> {
    @Query(value = "select * from SynchronizedWarehouses order by Name", nativeQuery = true)
    SynchronizedWarehouse[] getSyncWarehouses();

    @Query(value = "select * from SynchronizedWarehouses where Name = ?1", nativeQuery = true)
    SynchronizedWarehouse getSyncWarehouse(String name);
}
