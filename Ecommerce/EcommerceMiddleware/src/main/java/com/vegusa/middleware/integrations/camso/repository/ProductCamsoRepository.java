package com.vegusa.middleware.integrations.camso.repository;

import com.vegusa.middleware.integrations.camso.entity.ProductCamso;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface ProductCamsoRepository extends JpaRepository<ProductCamso, Long> {
    Optional<ProductCamso> findByPartNumber(String partNumber);

    List<ProductCamso> findBySync(Boolean sync);

    @Query(value = "select * from camso_products where sync = 0", nativeQuery = true)
    List<ProductCamso> getSyncProducts();

    @Query(value = "select * from camso_products where sync = 0", nativeQuery = true)
    ProductCamso[] getProducts();

    @Query(value = "select * from camso_products where DATE(updated_at) <> CURRENT_DATE", nativeQuery = true)
    List<ProductCamso> getOutdateProducts();
}
