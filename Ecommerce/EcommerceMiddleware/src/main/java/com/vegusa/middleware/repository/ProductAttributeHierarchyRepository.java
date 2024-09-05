package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.ProductAttributeHierarchy;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface ProductAttributeHierarchyRepository extends JpaRepository<ProductAttributeHierarchy, Long> {

    @Query(value = "select InterfaceId from productattributehierarchy pah where pah.ProductAttributeId = ?1 and pah.DataAreaId = ?2 order by pah.Priority", nativeQuery = true)
    List<String> getAttributeHierarchy(String productAttributeId, String dataAreaId);
}
