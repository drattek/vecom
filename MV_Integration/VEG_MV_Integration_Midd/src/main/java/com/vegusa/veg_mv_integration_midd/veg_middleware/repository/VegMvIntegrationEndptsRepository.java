package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.VegMvIntegrationEndpts;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface VegMvIntegrationEndptsRepository extends JpaRepository<VegMvIntegrationEndpts, Long>
{
    @Query(value = "select url from veg_ecomm_integration_endpts endpt where endpt.endpt_name = ?1", nativeQuery = true)
    String getEndPointMuitiVende(String endPointName);
}
