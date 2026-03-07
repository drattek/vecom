package com.vegusa.middleware.repository.erp;

import com.vegusa.middleware.model.erp.EcomProduct;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository
public interface EcomProductRepository extends JpaRepository<EcomProduct, Long> {
}
