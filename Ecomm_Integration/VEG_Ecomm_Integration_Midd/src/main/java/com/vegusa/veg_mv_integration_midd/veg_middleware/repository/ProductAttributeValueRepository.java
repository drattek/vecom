package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.ProductAttributeValue;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface ProductAttributeValueRepository extends JpaRepository<ProductAttributeValue, Long> {
    @Query(value = "select * from productattributevalue pav where pav.ProductAttributeId = ?1 and pav.ItemId = ?2 and pav.InterfaceId = ?3 and pav.DataAreaId = ?4", nativeQuery = true)
    ProductAttributeValue getProductAttributeValue(String productAttributeId, String ItemId, String InterfaceId, String DataAreaId);

    @Query(value = "select * from productattributevalue", nativeQuery = true)
    ProductAttributeValue[] test();
}
