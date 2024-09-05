package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SynchronizedItemInventory;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncItemInventoryRepository extends JpaRepository<SynchronizedItemInventory, Long> {
    @Query(value = "select * from synchronizediteminventory where IdEcom = ?1", nativeQuery = true)
    SynchronizedItemInventory getSyncItemInventory(String id);
}
