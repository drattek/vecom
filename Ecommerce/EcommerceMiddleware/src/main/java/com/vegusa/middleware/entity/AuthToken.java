package com.vegusa.middleware.entity;

import jakarta.persistence.*;

import java.util.Date;

@Entity
@Table(name = "AuthToken")
public class AuthToken
{
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId")
    private long recId;
    @Column(name = "ResponseId", nullable = false)
    private String responseId;
    @Column(name = "Status", nullable = false)
    private String status;
    @Column(name = "CipherAccessToken", nullable = false, length = 3000)
    private String cipherAccessToken;
    @Column(name = "AccessTokenExpiresAt", nullable = false)
    private Date accessTokenExpiresAt;
    @Column(name = "CipherRefreshToken", nullable = false)
    private String cipherRefreshToken;
    @Column(name = "RefreshTokenExpiresAt", nullable = false)
    private Date refreshTokenExpiresAt;
    @Column(name = "SecretKey", nullable = false)
    private String secretKey;
    @Column(name = "InitializationVector", nullable = false)
    private String initializationVector;
    @Column(name = "CreatedAt", nullable = false)
    private Date createdAt;
    @Column(name = "UpdatedAt", nullable = false)
    private Date updatedAt;
    @Column(name = "IntegrationCompany", nullable = false)
    private String integrationCompany;

    //getters
    public String getResponseId(){ return this.responseId; }
    public String getStatus(){ return this.status; }
    public String getCipherAccessToken(){ return this.cipherAccessToken; }
    public Date getAccessTokenExpiresAt(){ return this.accessTokenExpiresAt; }
    public String getCipherRefreshToken(){ return this.cipherRefreshToken; }
    public Date getRefreshTokenExpiresAt(){ return this.refreshTokenExpiresAt; }
    public String getSecretKey(){ return this.secretKey; }
    public String getInitializationVector(){ return this.initializationVector; }
    public Date getUpdatedAt(){ return this.updatedAt; }
    public Date getCreatedAt(){ return this.createdAt; }
    public String getIntegrationCompany(){ return this.integrationCompany; }

    //setters
    public void setResponseId(String responseId){ this.responseId = responseId; }
    public void setStatus(String status){ this.status = status; }
    public void setCipherAccessToken(String cipherAccessToken){ this.cipherAccessToken = cipherAccessToken; }
    public void setAccessTokenExpiresAt(Date accessTokenExpiresAt){ this.accessTokenExpiresAt = accessTokenExpiresAt; }
    public void setCipherRefreshToken(String cipherRefreshToken){ this.cipherRefreshToken = cipherRefreshToken; }
    public void setRefreshTokenExpiresAt(Date refreshTokenExpiresAt){ this.refreshTokenExpiresAt = refreshTokenExpiresAt; }
    public void setSecretKey(String secretKey){ this.secretKey = secretKey; }
    public void setInitializationVector(String initializationVector){ this.initializationVector = initializationVector; }
    public void setUpdatedAt(Date updatedAt){ this.updatedAt = updatedAt; }
    public void setCreatedAt(Date createdAt){ this.createdAt = createdAt; }
    public void setIntegrationCompany(String integrationCompany){ this.integrationCompany = integrationCompany; }
}
