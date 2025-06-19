package com.vegusa.middleware.integrations.jumpseller.dto;

public class AppInfo {
    private String name;
    private String code;
    private String currency;
    private String country;
    private String timezone;
    private String email;
    private String hooks_token;
    private String url;
    private String logo;
    private String weight_unit;
    private String subscription_status;
    private String subscription_plan;
    private String fb_pixel_id;
    private Address address;
    private String whatsapp_phone;
    private String mobile_app_version;
    private String checkout_version;

    private static class Address {
        private String address;
        private String city;
        private String postal;
        private String region;
        private String country;
        private String region_code;
        private String country_code;
        private Double latitude;
        private Double longitude;

        public Address() {}

        public Address(String address, String city, String postal, String region, String country, String region_code, String country_code, Double latitude, Double longitude) {
            this.address = address;
            this.city = city;
            this.postal = postal;
            this.region = region;
            this.country = country;
            this.region_code = region_code;
            this.country_code = country_code;
            this.latitude = latitude;
            this.longitude = longitude;
        }

        public String getAddress() {
            return address;
        }

        public void setAddress(String address) {
            this.address = address;
        }

        public String getCity() {
            return city;
        }

        public void setCity(String city) {
            this.city = city;
        }

        public String getPostal() {
            return postal;
        }

        public void setPostal(String postal) {
            this.postal = postal;
        }

        public String getRegion() {
            return region;
        }

        public void setRegion(String region) {
            this.region = region;
        }

        public String getCountry() {
            return country;
        }

        public void setCountry(String country) {
            this.country = country;
        }

        public String getRegion_code() {
            return region_code;
        }

        public void setRegion_code(String region_code) {
            this.region_code = region_code;
        }

        public String getCountry_code() {
            return country_code;
        }

        public void setCountry_code(String country_code) {
            this.country_code = country_code;
        }

        public Double getLatitude() {
            return latitude;
        }

        public void setLatitude(Double latitude) {
            this.latitude = latitude;
        }

        public Double getLongitude() {
            return longitude;
        }

        public void setLongitude(Double longitude) {
            this.longitude = longitude;
        }
    }

    public AppInfo() {
    }

    public AppInfo(String name, String code, String currency, String country, String timezone, String email, String hooks_token, String url, String logo, String weight_unit, String subscription_status, String subscription_plan, String fb_pixel_id, Address address, String whatsapp_phone, String mobile_app_version, String checkout_version) {
        this.name = name;
        this.code = code;
        this.currency = currency;
        this.country = country;
        this.timezone = timezone;
        this.email = email;
        this.hooks_token = hooks_token;
        this.url = url;
        this.logo = logo;
        this.weight_unit = weight_unit;
        this.subscription_status = subscription_status;
        this.subscription_plan = subscription_plan;
        this.fb_pixel_id = fb_pixel_id;
        this.address = address;
        this.whatsapp_phone = whatsapp_phone;
        this.mobile_app_version = mobile_app_version;
        this.checkout_version = checkout_version;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }

    public String getCode() {
        return code;
    }

    public void setCode(String code) {
        this.code = code;
    }

    public String getCurrency() {
        return currency;
    }

    public void setCurrency(String currency) {
        this.currency = currency;
    }

    public String getCountry() {
        return country;
    }

    public void setCountry(String country) {
        this.country = country;
    }

    public String getTimezone() {
        return timezone;
    }

    public void setTimezone(String timezone) {
        this.timezone = timezone;
    }

    public String getEmail() {
        return email;
    }

    public void setEmail(String email) {
        this.email = email;
    }

    public String getHooks_token() {
        return hooks_token;
    }

    public void setHooks_token(String hooks_token) {
        this.hooks_token = hooks_token;
    }

    public String getUrl() {
        return url;
    }

    public void setUrl(String url) {
        this.url = url;
    }

    public String getLogo() {
        return logo;
    }

    public void setLogo(String logo) {
        this.logo = logo;
    }

    public String getWeight_unit() {
        return weight_unit;
    }

    public void setWeight_unit(String weight_unit) {
        this.weight_unit = weight_unit;
    }

    public String getSubscription_status() {
        return subscription_status;
    }

    public void setSubscription_status(String subscription_status) {
        this.subscription_status = subscription_status;
    }

    public String getSubscription_plan() {
        return subscription_plan;
    }

    public void setSubscription_plan(String subscription_plan) {
        this.subscription_plan = subscription_plan;
    }

    public String getFb_pixel_id() {
        return fb_pixel_id;
    }

    public void setFb_pixel_id(String fb_pixel_id) {
        this.fb_pixel_id = fb_pixel_id;
    }

    public Address getAddress() {
        return address;
    }

    public void setAddress(Address address) {
        this.address = address;
    }

    public String getWhatsapp_phone() {
        return whatsapp_phone;
    }

    public void setWhatsapp_phone(String whatsapp_phone) {
        this.whatsapp_phone = whatsapp_phone;
    }

    public String getMobile_app_version() {
        return mobile_app_version;
    }

    public void setMobile_app_version(String mobile_app_version) {
        this.mobile_app_version = mobile_app_version;
    }

    public String getCheckout_version() {
        return checkout_version;
    }

    public void setCheckout_version(String checkout_version) {
        this.checkout_version = checkout_version;
    }
}