package com.vegusa.middleware.entity;

import jakarta.persistence.*;

import java.util.Date;

@Entity
@Table(name = "veg_ecomm_token_info")
public class TokenInfo
{
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private int id;
    @Column(nullable = false)
    private String id_mv;
    @Column(nullable = false)
    private String status;
    @Column(nullable = false, length = 3000)
    private String cipher_access_token;
    @Column(nullable = false)
    private Date access_token_expires_at;
    @Column(nullable = false)
    private String cipher_refresh_token;
    @Column(nullable = false)
    private Date refresh_token_expires_at;
    @Column(nullable = false)
    private String secret_key;
    @Column(nullable = false)
    private String initialization_vector;
    @Column(nullable = false)
    private Date updated_at;
    @Column(nullable = false)
    private String integration_company;

    @Column(nullable = false)
    private Date created_at;

    //getters
    public String getIdMv(){ return this.id_mv; }
    public String getStatus(){ return this.status; }
    public String getCipherAccessToken(){ return this.cipher_access_token; }
    public Date getAccessTokenExpiresAt(){ return this.access_token_expires_at; }
    public String getCipherRefreshToken(){ return this.cipher_refresh_token; }
    public Date getRefreshTokenExpiresAt(){ return this.refresh_token_expires_at; }
    public String getSecretKey(){ return this.secret_key; }
    public String getInitializationVector(){ return this.initialization_vector; }
    public Date getUpdatedAt(){ return this.updated_at; }
    public Date getCreatedAt(){ return this.created_at; }
    public String getIntegrationCompany(){ return this.integration_company; }

    //setters
    public void setIdMv(String id_mv){ this.id_mv = id_mv; }
    public void setStatus(String status){ this.status = status; }
    public void setCipherAccessToken(String cipher_access_token){ this.cipher_access_token = cipher_access_token; }
    public void setAccessTokenExpiresAt(Date access_token_expires_at){ this.access_token_expires_at = access_token_expires_at; }
    public void setCipherRefreshToken(String cipher_refresh_token){ this.cipher_refresh_token = cipher_refresh_token; }
    public void setRefreshTokenExpiresAt(Date refresh_token_expires_at){ this.refresh_token_expires_at = refresh_token_expires_at; }
    public void setSecretKey(String secret_key){ this.secret_key = secret_key; }
    public void setInitializationVector(String initialization_vector){ this.initialization_vector = initialization_vector; }
    public void setUpdatedAt(Date updated_at){ this.updated_at = updated_at; }
    public void setCreatedAt(Date created_at){ this.created_at = created_at; }
    public void setIntegrationCompany(String integration_company){ this.integration_company = integration_company; }
}
