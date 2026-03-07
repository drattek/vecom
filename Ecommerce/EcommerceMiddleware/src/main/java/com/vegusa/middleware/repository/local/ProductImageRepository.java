package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.ProductImage;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface ProductImageRepository extends JpaRepository<ProductImage, Long> {
    @Query(value = "select * from productimages where ItemId = :itemId and isActive = 1 order by Priority asc, ImageNumber asc", nativeQuery = true)
    List<ProductImage> getImages(@Param("itemId") String itemId);

    @Query(value = "select * from productImages where ItemId = :itemId and isActive = 1 and InterfaceId <> 'DYN'", nativeQuery = true)
    List<ProductImage> getOldImages(@Param("itemId") String itemId);

    @Query(value = "select * from productimages where ItemId = :itemId and BlobName = :blobName and isActive = 1 and InterfaceId = :interfaceId", nativeQuery = true)
    Optional<ProductImage> getImage(@Param("itemId") String itemId, @Param("blobName") String blobName, @Param("interfaceId") String interfaceId);
}
