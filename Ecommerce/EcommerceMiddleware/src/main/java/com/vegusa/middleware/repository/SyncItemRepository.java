package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SyncItem;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncItemRepository extends JpaRepository<SyncItem, Long>
{
    @Query(value = "select * from SyncItem where internal_code = ?1 and dataAreaId = ?2", nativeQuery = true)
    SyncItem getSyncItem(String itemId, String dataAreaId);

    @Query(value = "select * from SyncItem order by internal_code", nativeQuery = true)
    SyncItem[] getSyncItem();


}