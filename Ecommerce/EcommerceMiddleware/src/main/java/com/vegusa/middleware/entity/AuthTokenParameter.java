package com.vegusa.middleware.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "AuthTokenParameter")
public class AuthTokenParameter
{
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    @Column(name = "RecId")
    private int recId;
    @Column(name = "ClientId", nullable = false)
    private String clientId;
    @Column(name = "ClientSecret", nullable = false)
    private String clientSecret;
    @Column(name = "GrantTypeAuthCode", nullable = false)
    private String grantTypeAuthCode;
    @Column(name = "AuthorizationCode", nullable = false)
    private String authorizationCode;
    @Column(name = "GrantTypeRefreshToken", nullable = false)
    private String grantTypeRefreshToken;
    @Column(name = "IntegrationCompany", nullable = false)
    private String integrationCompany;

    //getters
    public String getClientId(){ return this.clientId; }
    public String getClientSecret(){ return this.clientSecret; }
    public String getGrantTypeAuthCode(){ return this.grantTypeAuthCode; }
    public String getAuthorizationCode(){ return this.authorizationCode; }
    public String getGrantTypeRefreshToken(){ return this.grantTypeRefreshToken; }
    public String getIntegrationCompany(){ return this.integrationCompany; }

    //setters
    public void setClientId(String clientId){ this.clientId = clientId; }
    public void setClientSecret(String clientSecret){ this.clientSecret = clientSecret; }
    public void setGrantTypeAuthCode(String grantTypeAuthCode){ this.grantTypeAuthCode = grantTypeAuthCode; }
    public void setAuthorizationCode(String authorizationCode){ this.authorizationCode = authorizationCode; }
    public void setGrantTypeRefreshToken(String grantTypeRefreshToken){ this.grantTypeRefreshToken = grantTypeRefreshToken; }
    public void setIntegrationCompany(String integrationCompany){ this.integrationCompany = integrationCompany; }
}
