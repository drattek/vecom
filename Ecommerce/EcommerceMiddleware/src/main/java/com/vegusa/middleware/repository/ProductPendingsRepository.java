package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.ProductPendings;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface ProductPendingsRepository extends JpaRepository<ProductPendings, Long> {
    Optional<ProductPendings> findByInternalCode(String internalCode);

    @Query(value = "select internal_code from product_pendings", nativeQuery = true)
    List<String> getAll();
}
