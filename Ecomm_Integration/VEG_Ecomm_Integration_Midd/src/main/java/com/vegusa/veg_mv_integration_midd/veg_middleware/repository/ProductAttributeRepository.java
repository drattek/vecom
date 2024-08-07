package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.ProductAttribute;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.ProductAttributeId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface ProductAttributeRepository  extends JpaRepository<ProductAttribute, ProductAttributeId> {
    @Query(value = "select * from productattribute pa where pa.ProductAttributeId = ?1", nativeQuery = true)
    ProductAttribute getProductAttribute(String productAttributeId);
}
