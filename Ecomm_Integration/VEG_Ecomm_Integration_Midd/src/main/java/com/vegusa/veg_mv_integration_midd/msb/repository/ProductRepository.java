package com.vegusa.veg_mv_integration_midd.msb.repository;

import com.vegusa.veg_mv_integration_midd.msb.entity.ECOMProduct;
import jakarta.persistence.QueryHint;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.jpa.repository.QueryHints;
import org.springframework.stereotype.Repository;

import java.util.stream.Stream;

import static org.hibernate.annotations.QueryHints.READ_ONLY;
import static org.hibernate.jpa.HibernateHints.HINT_CACHEABLE;
import static org.hibernate.jpa.HibernateHints.HINT_FETCH_SIZE;

@Repository
public interface ProductRepository extends JpaRepository<ECOMProduct, Long> {
    @QueryHints(value = {
            //@QueryHint(name = HINT_FETCH_SIZE, value = "" + 25),
            @QueryHint(name = HINT_FETCH_SIZE, value = "" + Integer.MIN_VALUE),
            @QueryHint(name = HINT_CACHEABLE, value = "false"),
            @QueryHint(name = READ_ONLY, value = "true")
    })
    @Query(value = "select * from ecomproducts", nativeQuery = true)
    Stream<ECOMProduct>  getStreamProductsToSynchronize();

    @Query(value = "select * from ecomproducts where articulo like 'MSB-_______' order by articulo", nativeQuery = true)
    ECOMProduct[] getProductsToSynchronize();

}
