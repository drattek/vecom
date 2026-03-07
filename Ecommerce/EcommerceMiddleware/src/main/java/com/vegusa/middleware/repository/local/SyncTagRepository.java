package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.SyncTag;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

public interface SyncTagRepository extends JpaRepository<SyncTag, Long>{
    @Query(value = "select * from SyncTag where Name = ?1 and DataAreaId = ?2", nativeQuery = true)
    SyncTag getSyncTag(String name, String dataAreaId);
}
