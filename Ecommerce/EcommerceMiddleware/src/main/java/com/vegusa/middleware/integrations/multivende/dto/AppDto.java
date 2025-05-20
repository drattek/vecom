package com.vegusa.middleware.integrations.multivende.dto;

import com.fasterxml.jackson.annotation.JsonInclude;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class AppDto {
    private String id;
    private String merchantId;
    private DeveloperApp developerApp;
    private MarketplaceConnection marketplaceConnection;
}