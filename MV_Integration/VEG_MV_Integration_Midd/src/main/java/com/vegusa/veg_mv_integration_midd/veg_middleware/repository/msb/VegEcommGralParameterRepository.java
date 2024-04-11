package com.vegusa.veg_mv_integration_midd.veg_middleware.repository.msb;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.msb.VegEcommGralParameter;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface VegEcommGralParameterRepository extends JpaRepository<VegEcommGralParameter, Long> {
    @Query(value = "select * from veg_ecomm_gral_parameters vegp where vegp.parameter_name = ?1", nativeQuery = true)
    VegEcommGralParameter getMiddlewareGeneralParameter(String parameterName);


}
