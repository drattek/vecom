package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.AuthToken;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface AuthTokenRepository extends JpaRepository<AuthToken, Integer>
{
    @Query(value = "select * from AuthToken where integration_company = ?1",nativeQuery = true)
    AuthToken getAuthToken(String integrationCompany);
}