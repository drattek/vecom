package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.ItemImages;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface ItemImagesRepository extends JpaRepository<ItemImages, Long>
{
    @Query(value = "select * from ItemImages where DataAreaId = ?1 order by ItemId, Source desc", nativeQuery = true)
    ItemImages[] getItemImages(String dataAreaId);
}
