package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.VegEcomSynchronizedImages;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository
public interface VegEcomSynchronizedImagesRepository extends JpaRepository<VegEcomSynchronizedImages, Long> {
}
