package com.vegusa.middleware.integrations.camso.repository;

import com.vegusa.middleware.integrations.camso.entity.ImageCamso;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.Optional;

@Repository
public interface ImageCamsoRepository extends JpaRepository<ImageCamso, Long> {
    Optional<ImageCamso> findByPartNumber(String partNumber);
}
