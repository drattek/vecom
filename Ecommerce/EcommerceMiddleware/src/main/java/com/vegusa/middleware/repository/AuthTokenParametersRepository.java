package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.AuthTokenParameters;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface AuthTokenParametersRepository extends JpaRepository<AuthTokenParameters, Integer> {
    @Query(value = "select * from AuthTokenParameters where integration_company = ?1",nativeQuery = true)
    AuthTokenParameters getTokenInfoParameters(String integrationCompany);
}

