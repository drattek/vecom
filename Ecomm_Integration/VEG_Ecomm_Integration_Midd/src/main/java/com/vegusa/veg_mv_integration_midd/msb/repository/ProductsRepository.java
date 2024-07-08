package com.vegusa.veg_mv_integration_midd.msb.repository;

import com.vegusa.veg_mv_integration_midd.msb.entity.EcommProducts;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;
import org.springframework.data.jpa.repository.QueryHints;
import jakarta.persistence.QueryHint;

import java.util.stream.Stream;

import static org.hibernate.annotations.QueryHints.READ_ONLY;
import static org.hibernate.jpa.HibernateHints.HINT_CACHEABLE;
import static org.hibernate.jpa.HibernateHints.HINT_FETCH_SIZE;

@Repository
public interface ProductsRepository extends JpaRepository<EcommProducts, Long>
{

    @QueryHints(value = {
         //   @QueryHint(name = HINT_FETCH_SIZE, value = "" + Integer.MIN_VALUE),
            @QueryHint(name = HINT_FETCH_SIZE, value = "" + 25),
            @QueryHint(name = HINT_CACHEABLE, value = "false"),
            @QueryHint(name = READ_ONLY, value = "true")
    })
    @Query(value = "select distinct ep.Name, ep.NameAlias, ep.SearchName, ep.Description, ep.ItemId, ep.InventTable_modifieddatetime " +
        "from ecommproducts ep where ep.ItemId like 'MSB-_______' order by ep.ItemId ",
        nativeQuery = true)
    Stream<Object[]> getProductsToSynchronize();
}
