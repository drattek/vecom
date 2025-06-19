package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SyncItem;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface SyncItemRepository extends JpaRepository<SyncItem, Long>
{
    @Query(value = "select * from SyncItem where InternalCode = ?1 and DataAreaId = ?2", nativeQuery = true)
    SyncItem getSyncItem(String itemId, String dataAreaId);

    @Query(value = "SELECT * FROM SyncItem where InternalCode IN :ids and DataAreaId = :dataAreaId", nativeQuery = true)
    SyncItem[] getSyncItems(@Param("ids")List<String> ids, @Param("dataAreaId") String dataAreaId);

    @Query(value = "select * from SyncItem order by InternalCode", nativeQuery = true)
    SyncItem[] getSyncItem();

    @Query(value = "select * from syncItem where Alias = ?1 and Code = ?2 and DataAreaId = ?3 limit 1", nativeQuery = true)
    SyncItem getSyncItemByName(String name, String code, String dataAreaId);

    @Query(value = "select * from syncItem order by InternalCode limit :limit offset :offset", nativeQuery = true)
    SyncItem[] getPagedItems(@Param("limit") int limit, @Param("offset") int offset);
}