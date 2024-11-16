package com.vegusa.middleware.entity;

import jakarta.persistence.*;

import java.util.Date;

@Entity
@Table(name = "authtoken")
public class AuthToken {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId", nullable = false)
    private Long id;

    @Column(name = "AccessTokenExpiresAt", nullable = false)
    private Date accessTokenExpiresAt;

    @Column(name = "CipherAccessToken", nullable = false, length = 3000)
    private String cipherAccessToken;

    @Column(name = "CipherRefreshToken", nullable = false)
    private String cipherRefreshToken;

    @Column(name = "CreatedAt", nullable = false)
    private Date createdAt;

    @Column(name = "ResponseId", nullable = false)
    private String responseId;

    @Column(name = "InitializationVector", nullable = false)
    private String initializationVector;

    @Column(name = "RefreshTokenExpiresAt", nullable = false)
    private Date refreshTokenExpiresAt;

    @Column(name = "SecretKey", nullable = false)
    private String secretKey;

    @Column(name = "Status", nullable = false)
    private String status;

    @Column(name = "UpdatedAt", nullable = false)
    private Date updatedAt;

    @Column(name = "IntegrationCompany", nullable = false, length = 50)
    private String integrationCompany;

    public Long getId() {
        return id;
    }

    public void setId(Long id) {
        this.id = id;
    }

    public Date getAccessTokenExpiresAt() {
        return accessTokenExpiresAt;
    }

    public void setAccessTokenExpiresAt(Date accessTokenExpiresAt) {
        this.accessTokenExpiresAt = accessTokenExpiresAt;
    }

    public String getCipherAccessToken() {
        return cipherAccessToken;
    }

    public void setCipherAccessToken(String cipherAccessToken) {
        this.cipherAccessToken = cipherAccessToken;
    }

    public String getCipherRefreshToken() {
        return cipherRefreshToken;
    }

    public void setCipherRefreshToken(String cipherRefreshToken) {
        this.cipherRefreshToken = cipherRefreshToken;
    }

    public Date getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(Date createdAt) {
        this.createdAt = createdAt;
    }

    public String getResponseId() {
        return responseId;
    }

    public void setResponseId(String responseId) {
        this.responseId = responseId;
    }

    public String getInitializationVector() {
        return initializationVector;
    }

    public void setInitializationVector(String initializationVector) {
        this.initializationVector = initializationVector;
    }

    public Date getRefreshTokenExpiresAt() {
        return refreshTokenExpiresAt;
    }

    public void setRefreshTokenExpiresAt(Date refreshTokenExpiresAt) {
        this.refreshTokenExpiresAt = refreshTokenExpiresAt;
    }

    public String getSecretKey() {
        return secretKey;
    }

    public void setSecretKey(String secretKey) {
        this.secretKey = secretKey;
    }

    public String getStatus() {
        return status;
    }

    public void setStatus(String status) {
        this.status = status;
    }

    public Date getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(Date updatedAt) {
        this.updatedAt = updatedAt;
    }

    public String getIntegrationCompany() {
        return integrationCompany;
    }

    public void setIntegrationCompany(String integrationCompany) {
        this.integrationCompany = integrationCompany;
    }

}