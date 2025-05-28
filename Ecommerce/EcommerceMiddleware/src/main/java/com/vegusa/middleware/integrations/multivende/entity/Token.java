package com.vegusa.middleware.integrations.multivende.entity;

import jakarta.persistence.*;

import java.util.Date;

@Entity
@Table(name = "authtoken")
public class Token {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId")
    private Long recId;

    @Column(name = "AccessTokenExpiresAt")
    private Date accessTokenExpiresAt;

    @Column(name = "CipherAccessToken")
    private String cipherAccessToken;

    @Column(name = "CipherRefreshToken")
    private String cipherRefreshToken;

    @Column(name = "CreatedAt")
    private Date createdAt;

    @Column(name = "ResponseId")
    private String responseId;

    @Column(name = "InitializationVector")
    private String initializationVector;

    @Column(name = "RefreshTokenExpiresAt")
    private Date refreshTokenExpiresAt;

    @Column(name = "SecretKey")
    private String secretKey;

    @Column(name = "Status")
    private String status;

    @Column(name = "UpdatedAt")
    private Date updatedAt;

    @Column(name = "IntegrationCompany")
    private String integrationCompany;

    public Long getRecId() {
        return recId;
    }

    public void setRecId(Long recId) {
        this.recId = recId;
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
