package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.VegEcomIntegrationEndpts;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface VegEcomvIntegrationEndptsRepository extends JpaRepository<VegEcomIntegrationEndpts, Long>
{
    @Query(value = "select url from veg_ecomm_integration_endpts endpt where endpt.endpt_name = ?1 and endpt.integration_company = ?2", nativeQuery = true)
    String getIntegrationEndPoint(String endPointName, String integrationCompany);
}
