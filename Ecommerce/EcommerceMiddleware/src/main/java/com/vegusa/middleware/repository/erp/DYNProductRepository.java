package com.vegusa.middleware.repository.erp;

import com.vegusa.middleware.model.erp.DYNProduct;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface DYNProductRepository extends JpaRepository<DYNProduct, Long> {
    @Query(value = "select * from dyn.ECOMProducts order by articulo", nativeQuery = true)
    DYNProduct[] getDYNProducts();
}
