package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegEcomSynchronizedCategories;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface VegEcomSynchronizedCategoriesRepository extends JpaRepository<VegEcomSynchronizedCategories, Long> {
    @Query(value = "select * from veg_ecom_synchronized_categories vesc where vesc.name = ?1 and vesc.veg_company = ?2", nativeQuery = true)
    VegEcomSynchronizedCategories getSynchronizedCategory(String categoryName, String company);
}
