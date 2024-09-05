package com.vegusa.msb.repository;

import com.vegusa.msb.entity.MSBProduct;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface MSBProductRepository extends JpaRepository<MSBProduct, Long> {
    @Query(value = "select * from ecomproducts where articulo like 'MSB-_______' order by articulo", nativeQuery = true)
    MSBProduct[] getDYNProducts();
}
