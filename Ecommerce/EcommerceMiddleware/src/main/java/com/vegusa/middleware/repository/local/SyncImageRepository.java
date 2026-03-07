package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.SyncImage;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncImageRepository extends JpaRepository<SyncImage, Long> {
    @Query(value = "select * from syncimage where OriginalFileName like %:blobName% and ProductPictureSetId = :albumId", nativeQuery = true)
    SyncImage getSyncImage(@Param("blobName") String blobName, @Param("albumId") String albumId);

    @Query(value = "select * from syncimage where OriginalFileName like %:blobName% and ProductPictureSetId IS NULL", nativeQuery = true)
    SyncImage getSyncImageDefault(@Param("blobName") String blobName);
}
