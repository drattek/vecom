package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.ItemImages;
import jakarta.persistence.QueryHint;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.jpa.repository.QueryHints;
import org.springframework.stereotype.Repository;

import java.util.function.Supplier;
import java.util.stream.Stream;

import static org.hibernate.annotations.QueryHints.READ_ONLY;
import static org.hibernate.jpa.HibernateHints.HINT_CACHEABLE;
import static org.hibernate.jpa.HibernateHints.HINT_FETCH_SIZE;

@Repository
public interface ItemImagesRepository extends JpaRepository<ItemImages, Long>
{
    @QueryHints(value = {
            @QueryHint(name = HINT_FETCH_SIZE, value = "" + Integer.MIN_VALUE),
            @QueryHint(name = HINT_CACHEABLE, value = "false"),
            @QueryHint(name = READ_ONLY, value = "true")
    })
    @Query(value = "select * from ItemImages where veg_business_unit = ?1 order by internal_code", nativeQuery = true)
    Supplier<Stream<ItemImages>> getItemImages(String dataAreaId);

    @Query(value = "select id_mv from ItemImages where veg_business_unit = ?1 order by internal_code limit 1", nativeQuery = true)
    String getFstItemImageId(String dataAreaId);
}
