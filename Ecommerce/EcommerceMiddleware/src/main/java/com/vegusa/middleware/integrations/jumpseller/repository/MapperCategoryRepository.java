package com.vegusa.middleware.integrations.jumpseller.repository;

import com.vegusa.middleware.integrations.jumpseller.entity.MapperCategory;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.Optional;

public interface MapperCategoryRepository extends JpaRepository<MapperCategory, Long> {
    Optional<MapperCategory> findByItemId(String itemId);
}
