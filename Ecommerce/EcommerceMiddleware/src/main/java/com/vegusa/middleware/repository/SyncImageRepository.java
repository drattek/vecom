package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SyncImage;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncImageRepository extends JpaRepository<SyncImage, Long> { }
