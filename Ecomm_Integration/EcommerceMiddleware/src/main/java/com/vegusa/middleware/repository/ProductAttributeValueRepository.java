package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.ProductAttributeValue;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface ProductAttributeValueRepository extends JpaRepository<ProductAttributeValue, Long> {
    @Query(value = "select * from productattributevalue pav where pav.ProductAttributeId = ?1 and pav.ItemId = ?2 and pav.InterfaceId = ?3 and pav.DataAreaId = ?4", nativeQuery = true)
    ProductAttributeValue getProductAttributeValue(String productAttributeId, String ItemId, String InterfaceId, String DataAreaId);

    @Modifying
    @Query(value = "delete from productattributevalue pav where pav.ProductAttributeId = ?1 and pav.ItemId = ?2 and pav.InterfaceId = ?3 and pav.DataAreaId = ?4", nativeQuery = true)
    void deleteProductAttributeValue(String productAttributeId, String ItemId, String InterfaceId, String DataAreaId);

    @Query(value = "select distinct ItemId from productattributevalue pav where pav.DataAreaId = ?1 order by pav.ItemId limit 1", nativeQuery = true)
    List<String> getItemIdList(String DataAreaId);

}
