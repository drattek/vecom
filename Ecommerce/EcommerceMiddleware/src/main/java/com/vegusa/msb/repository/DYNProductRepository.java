package com.vegusa.msb.repository;

import com.vegusa.msb.entity.DYNProduct;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface DYNProductRepository extends JpaRepository<DYNProduct, Long> {
    @Query(value = "select * from ecomproducts where articulo like 'MSB-_______' order by articulo", nativeQuery = true)
    DYNProduct[] getDYNProducts();
}
