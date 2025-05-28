package com.vegusa.middleware.integrations.multivende.repository;

import com.vegusa.middleware.integrations.multivende.entity.Token;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.Optional;

public interface TokenRepository extends JpaRepository<Token, Long> {
    Optional<Token> findByIntegrationCompany(String integrationCompany);
}
