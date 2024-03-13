package com.vegusa.veg_mv_integration_midd.oauth2_0.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "veg_mv_token_info_parameters")
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
    private String authorizationCode;
    @Column(nullable = false)
    private String grant_type_refresh_token;

    //getters
    public String getClientId(){ return this.client_id; }
    public String getClientSecret(){ return this.client_secret; }
    public String getGrantTypeAuthCode(){ return this.grant_type_auth_code; }
    public String getAuthorizationCode(){ return this.authorizationCode; }
    public String getGrantTypeRefreshToken(){ return this.grant_type_refresh_token; }

    //setters
    public void setClientId(String client_id){ this.client_id = client_id; }
    public void setClientSecret(String client_secret){ this.client_secret = client_secret; }
    public void setGrantTypeAuthCode(String grant_type_auth_code){ this.grant_type_auth_code = grant_type_auth_code; }
    public void setAuthorizationCode(String authorizationCode){ this.authorizationCode = authorizationCode; }
    public void setGrantTypeRefreshToken(String grant_type_refresh_token){ this.grant_type_refresh_token = grant_type_refresh_token; }
}
