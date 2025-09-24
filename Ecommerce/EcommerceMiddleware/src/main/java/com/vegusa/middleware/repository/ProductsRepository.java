package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.Products;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.Optional;

@Repository
public interface ProductsRepository extends JpaRepository<Products, Long> {
    @Query(value = "select SkipNull from product where ItemId = :itemId and DataAreaId = :dataAreaId limit 1", nativeQuery = true)
    String getSkipNull(@Param("itemId") String itemId, @Param("dataAreaId") String dataAreaId);

    @Query(value = "select * from product where ItemId = :itemId and InterfaceId = :interfaceId", nativeQuery = true)
    Optional<Products> getProduct(@Param("itemId") String itemId, @Param("interfaceId") String interfaceId);
}
