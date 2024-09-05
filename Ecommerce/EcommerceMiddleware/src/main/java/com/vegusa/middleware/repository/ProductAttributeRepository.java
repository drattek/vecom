package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.ProductAttribute;
import com.vegusa.middleware.entity.ProductAttributeId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface ProductAttributeRepository  extends JpaRepository<ProductAttribute, ProductAttributeId> {
    @Query(value = "select ProductAttributeId from productattribute pa where pa.DataAreaId = ?1", nativeQuery = true)
    List<String> getProductAttribute(String dataAreaId);

    @Query(value = "select * from productattribute pa where pa.ProductAttributeId = ?1 and pa.DataAreaId = ?2", nativeQuery = true)
    ProductAttribute getProductAttribute(String productAttributeId, String dataAreaId);
}
