package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.Token;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.Optional;

public interface TokenRepository extends JpaRepository<Token, Long> {
    Optional<Token> findByIntegrationCompany(String integrationCompany);
}
