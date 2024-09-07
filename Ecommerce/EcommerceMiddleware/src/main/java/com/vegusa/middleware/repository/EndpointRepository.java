package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.Endpoint;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface EndpointRepository extends JpaRepository<Endpoint, Long>
{
    @Query(value = "select url from Endpoint where endpt_name = ?1 and integration_company = ?2", nativeQuery = true)
    String getEndpointUrl(String endPointName, String integrationCompany);
}
