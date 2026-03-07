package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.PriceListParameter;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface PriceListParameterRepository extends JpaRepository<PriceListParameter, Long> {
    @Query(value = "select * from pricelistparameter where Name = ?1 and DataAreaId = ?2", nativeQuery = true)
    PriceListParameter getPriceListParameter(String name, String dataAreaId);
}
