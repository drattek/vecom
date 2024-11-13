package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.AuthTokenParameter;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface AuthTokenParameterRepository extends JpaRepository<AuthTokenParameter, Integer> {
    @Query(value = "select * from AuthTokenParameter where IntegrationCompany = ?1",nativeQuery = true)
    AuthTokenParameter getTokenInfoParameters(String integrationCompany);
}

