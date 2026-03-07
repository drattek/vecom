package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.SyncItemInventory;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncItemInventoryRepository extends JpaRepository<SyncItemInventory, Long> {
    @Query(value = "select * from SyncItemInventory where IdEcom = ?1", nativeQuery = true)
    SyncItemInventory getSyncItemInventory(String id);
}
