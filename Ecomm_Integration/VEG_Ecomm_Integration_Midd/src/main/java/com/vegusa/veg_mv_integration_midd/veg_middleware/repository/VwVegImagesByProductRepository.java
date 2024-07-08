package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VwVegImagesByProduct;
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
public interface VwVegImagesByProductRepository extends JpaRepository<VwVegImagesByProduct, Long>
{
    @QueryHints(value = {
            @QueryHint(name = HINT_FETCH_SIZE, value = "" + Integer.MIN_VALUE),
            @QueryHint(name = HINT_CACHEABLE, value = "false"),
            @QueryHint(name = READ_ONLY, value = "true")
    })
    @Query(value = "select * from vw_veg_images_by_product vibp order by vibp.internal_code", nativeQuery = true)
    Stream<VwVegImagesByProduct> getSynchronizedProductsWithImages();

    @Query(value = "select id_mv from vw_veg_images_by_product vibp order by vibp.internal_code limit 1", nativeQuery = true)
    String getFstSyncProductIDWithImage();
}
