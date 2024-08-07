package com.vegusa.veg_mv_integration_midd.msb.repository;

import com.vegusa.veg_mv_integration_midd.msb.entity.ECOMProduct;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface ProductRepository extends JpaRepository<ECOMProduct, Long> {
    @Query(value = "select * from ecomproducts where articulo like 'MSB-_______' order by articulo", nativeQuery = true)
    ECOMProduct[] getDYNProducts();
}
