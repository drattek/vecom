package com.vegusa.middleware.integrations.mercadolibre.dto.product;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public class PictureMeliDTO {
    @JsonProperty("id")
    private String id;

    @JsonProperty("url")
    private String url;

    @JsonProperty("secure_url")
    private String secureUrl;

    @JsonProperty("size")
    private String size;

    @JsonProperty("max_size")
    private String maxSise;

    @JsonProperty("quality")
    private String quality;

    @JsonProperty("source")
    private String source;

    public PictureMeliDTO() {}

    public PictureMeliDTO(String id, String url, String secureUrl, String size, String maxSise, String quality, String source) {
        this.id = id;
        this.url = url;
        this.secureUrl = secureUrl;
        this.size = size;
        this.maxSise = maxSise;
        this.quality = quality;
        this.source = source;
    }

    public String getId() {
        return id;
    }

    public void setId(String id) {
        this.id = id;
    }

    public String getUrl() {
        return url;
    }

    public void setUrl(String url) {
        this.url = url;
    }

    public String getSecureUrl() {
        return secureUrl;
    }

    public void setSecureUrl(String secureUrl) {
        this.secureUrl = secureUrl;
    }

    public String getSize() {
        return size;
    }

    public void setSize(String size) {
        this.size = size;
    }

    public String getMaxSise() {
        return maxSise;
    }

    public void setMaxSise(String maxSise) {
        this.maxSise = maxSise;
    }

    public String getQuality() {
        return quality;
    }

    public void setQuality(String quality) {
        this.quality = quality;
    }

    public String getSource() {
        return source;
    }

    public void setSource(String source) {
        this.source = source;
    }
}
