package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.VegEcomSynchronizedBrands;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface VegEcomSynchronizedBrandsRepository extends JpaRepository<VegEcomSynchronizedBrands, Long> {
    @Query(value = "select * from veg_ecom_synchronized_brands vesb where vesb.name = ?1 and vesb.veg_company = ?2", nativeQuery = true)
    VegEcomSynchronizedBrands getSynchronizedBrand(String brandName, String company);
}
