package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.Endpoint;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface EndpointRepository extends JpaRepository<Endpoint, Long>
{
    @Query(value = "select Url from Endpoint where Name = ?1 and IntegrationCompany = ?2", nativeQuery = true)
    String getEndpointUrl(String endpointName, String integrationCompany);
}
