package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.ProductAttributeValues;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.util.List;
import java.util.Optional;

public interface ProductAttributeValuesRepository extends JpaRepository<ProductAttributeValues, Long> {
    @Query(value = "select * from productattributevalue where ItemId = :itemId and ProductAttributeId = :attribute and Value is not null limit 1", nativeQuery = true)
    Optional<ProductAttributeValues> getAttribute(@Param("itemId") String itemId, @Param("attribute") String attribute);

    @Query(value = "select * from productattributevalue where ItemId = :itemId and ProductAttributeId IN ('DESCRIPTION', 'META_DESCRIPTION') and InterfaceId = 'DYN'", nativeQuery = true)
    List<ProductAttributeValues> getDescriptions(@Param("itemId") String itemId);
}
