package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegEcommSynchronizedImage;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository

public interface VegEcommSynchronizedImageRepository  extends JpaRepository<VegEcommSynchronizedImage, Long> {
}
