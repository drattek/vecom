package com.vegusa.middleware.integrations.jumpseller.repository;

import com.vegusa.middleware.integrations.jumpseller.entity.SyncCategoryJumpseller;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository
public interface SyncCategoryRepositoryJumpseller extends JpaRepository<SyncCategoryJumpseller, Long> {
}
