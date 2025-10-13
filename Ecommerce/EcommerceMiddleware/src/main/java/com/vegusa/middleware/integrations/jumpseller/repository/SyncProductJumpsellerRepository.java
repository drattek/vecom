package com.vegusa.middleware.integrations.jumpseller.repository;

import com.vegusa.middleware.integrations.jumpseller.entity.SyncJumpsellerProduct;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.Optional;

@Repository
public interface SyncProductJumpsellerRepository extends JpaRepository<SyncJumpsellerProduct, Long> {
    @Query(value = "select * from syncItemJumpseller where DataAreaId = ?1 order by InternalCode", nativeQuery = true)
    SyncJumpsellerProduct[] getSyncProducts(String dataAreaId);

    @Query(value = "select * from syncItemJumpseller where ResponseId = ?1 limit 1", nativeQuery = true)
    SyncJumpsellerProduct getSyncById(Long id);

    Optional<SyncJumpsellerProduct> findByResponseId(long responseId);

    Optional<SyncJumpsellerProduct> findByInternalCode(String internalCode);
}
