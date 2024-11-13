package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SyncItem;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncItemRepository extends JpaRepository<SyncItem, Long>
{
    @Query(value = "select * from SyncItem where InternalCode = ?1 and DataAreaId = ?2", nativeQuery = true)
    SyncItem getSyncItem(String itemId, String dataAreaId);

    @Query(value = "select * from SyncItem order by InternalCode", nativeQuery = true)
    SyncItem[] getSyncItem();


}