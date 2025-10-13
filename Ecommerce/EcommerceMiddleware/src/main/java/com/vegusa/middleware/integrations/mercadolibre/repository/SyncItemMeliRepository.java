package com.vegusa.middleware.integrations.mercadolibre.repository;

import com.vegusa.middleware.integrations.mercadolibre.entity.SyncItemMeli;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface SyncItemMeliRepository extends JpaRepository<SyncItemMeli, Long> {
    @Query(value = "select * from syncitemmeli where DataAreaId = :dataAreaId", nativeQuery = true)
    SyncItemMeli[] getItems(@Param("dataAreaId") String dataAreaId);

    Optional<SyncItemMeli> findByInternalCode(String internalCode);

    @Query(value = "select * from syncitemmeli where RecId > 1831 and Status <> 'under_review'", nativeQuery = true)
    List<SyncItemMeli> getNewItems();
}
