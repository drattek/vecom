package com.vegusa.middleware.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "veg_ecomm_token_info_parameters")
public class TokenInfoParameters
{
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private int id;
    @Column(nullable = false)
    private String client_id;
    @Column(nullable = false)
    private String client_secret;
    @Column(nullable = false)
    private String grant_type_auth_code;
    @Column(nullable = false)
    private String authorization_code;
    @Column(nullable = false)
    private String grant_type_refresh_token;
    @Column(nullable = false)
    private String integration_company;

    //getters
    public String getClientId(){ return this.client_id; }
    public String getClientSecret(){ return this.client_secret; }
    public String getGrantTypeAuthCode(){ return this.grant_type_auth_code; }
    public String getAuthorizationCode(){ return this.authorization_code; }
    public String getGrantTypeRefreshToken(){ return this.grant_type_refresh_token; }
    public String getIntegrationCompany(){ return this.integration_company; }

    //setters
    public void setClientId(String client_id){ this.client_id = client_id; }
    public void setClientSecret(String client_secret){ this.client_secret = client_secret; }
    public void setGrantTypeAuthCode(String grant_type_auth_code){ this.grant_type_auth_code = grant_type_auth_code; }
    public void setAuthorizationCode(String authorization_code){ this.authorization_code = authorization_code; }
    public void setGrantTypeRefreshToken(String grant_type_refresh_token){ this.grant_type_refresh_token = grant_type_refresh_token; }
    public void setIntegrationCompany(String integration_company){ this.integration_company = integration_company; }
}
