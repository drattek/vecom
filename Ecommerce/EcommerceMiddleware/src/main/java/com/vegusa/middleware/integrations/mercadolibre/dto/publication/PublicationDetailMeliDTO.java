package com.vegusa.middleware.integrations.mercadolibre.dto.publication;

public class PublicationDetailMeliDTO {
    private String id;
    private String[] notAvailableInCategories;

    public static class Configuration {
        private String name;
        private String listingExposure;
        private Boolean requiresPicture;
        private Long maxStockPerItem;
        private String mercadoPago;
    }
}
