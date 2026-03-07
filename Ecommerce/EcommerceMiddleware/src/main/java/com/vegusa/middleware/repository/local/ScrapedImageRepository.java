package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.ScrapedImage;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface ScrapedImageRepository extends JpaRepository<ScrapedImage, Long> {
   @Query(value = "select * from scrapedimage where Source = ?1 order by ItemId", nativeQuery = true)
   ScrapedImage[] getScrapedImage(String source);
}
