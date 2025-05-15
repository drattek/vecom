package com.vegusa.middleware.integrations.jumpseller.dto;

public class JumpsellerProductDto {
    private Product product;

    public  JumpsellerProductDto(){}

    public JumpsellerProductDto(Product product) {
        this.product = product;
    }

    public Product getProduct() {
        return product;
    }

    public void setProduct(Product product) {
        this.product = product;
    }

    public static class Product {
        private Long id;
        private String name;
        private String page_title;
        private String description;
        private String meta_description;
        private String type;
        private int days_to_expire;
        private double price;
        private double discount;
        private double weight;
        private int stock;
        private boolean stock_unlimited;
        private int stock_threshold;
        private boolean stock_notification;
        private Double cost_per_item;
        private Double compare_at_price;
        private int minimum_quantity;
        private int maximum_quantity;
        private String sku;
        private String brand;
        private String barcode;
        private String google_product_category;
        private boolean featured;
        private boolean shipping_required;
        private boolean reviews_enabled;
        private String status;
        private String created_at;
        private String updated_at;
        private String package_format;
        private double length;
        private double width;
        private double height;
        private double diameter;
        private String permalink;
        private Category[] categories;

        public  Product(){}

        public Product(Long id, String name, String page_title, String description, String meta_description, String type, int days_to_expire, double price, double discount, double weight, int stock, boolean stock_unlimited, int stock_threshold, boolean stock_notification, Double cost_per_item, Double compare_at_price, int minimum_quantity, int maximum_quantity, String sku, String brand, String barcode, String google_product_category, boolean featured, boolean shipping_required, boolean reviews_enabled, String status, String created_at, String updated_at, String package_format, double length, double width, double height, double diameter, String permalink, Category[] categories) {
            this.id = id;
            this.name = name;
            this.page_title = page_title;
            this.description = description;
            this.meta_description = meta_description;
            this.type = type;
            this.days_to_expire = days_to_expire;
            this.price = price;
            this.discount = discount;
            this.weight = weight;
            this.stock = stock;
            this.stock_unlimited = stock_unlimited;
            this.stock_threshold = stock_threshold;
            this.stock_notification = stock_notification;
            this.cost_per_item = cost_per_item;
            this.compare_at_price = compare_at_price;
            this.minimum_quantity = minimum_quantity;
            this.maximum_quantity = maximum_quantity;
            this.sku = sku;
            this.brand = brand;
            this.barcode = barcode;
            this.google_product_category = google_product_category;
            this.featured = featured;
            this.shipping_required = shipping_required;
            this.reviews_enabled = reviews_enabled;
            this.status = status;
            this.created_at = created_at;
            this.updated_at = updated_at;
            this.package_format = package_format;
            this.length = length;
            this.width = width;
            this.height = height;
            this.diameter = diameter;
            this.permalink = permalink;
            this.categories = categories;
        }

        public Long getId() {
            return id;
        }

        public void setId(Long id) {
            this.id = id;
        }

        public String getName() {
            return name;
        }

        public void setName(String name) {
            this.name = name;
        }

        public String getPage_title() {
            return page_title;
        }

        public void setPage_title(String page_title) {
            this.page_title = page_title;
        }

        public String getDescription() {
            return description;
        }

        public void setDescription(String description) {
            this.description = description;
        }

        public String getMeta_description() {
            return meta_description;
        }

        public void setMeta_description(String meta_description) {
            this.meta_description = meta_description;
        }

        public String getType() {
            return type;
        }

        public void setType(String type) {
            this.type = type;
        }

        public int getDays_to_expire() {
            return days_to_expire;
        }

        public void setDays_to_expire(int days_to_expire) {
            this.days_to_expire = days_to_expire;
        }

        public double getPrice() {
            return price;
        }

        public void setPrice(double price) {
            this.price = price;
        }

        public double getDiscount() {
            return discount;
        }

        public void setDiscount(double discount) {
            this.discount = discount;
        }

        public double getWeight() {
            return weight;
        }

        public void setWeight(double weight) {
            this.weight = weight;
        }

        public int getStock() {
            return stock;
        }

        public void setStock(int stock) {
            this.stock = stock;
        }

        public boolean isStock_unlimited() {
            return stock_unlimited;
        }

        public void setStock_unlimited(boolean stock_unlimited) {
            this.stock_unlimited = stock_unlimited;
        }

        public int getStock_threshold() {
            return stock_threshold;
        }

        public void setStock_threshold(int stock_threshold) {
            this.stock_threshold = stock_threshold;
        }

        public boolean isStock_notification() {
            return stock_notification;
        }

        public void setStock_notification(boolean stock_notification) {
            this.stock_notification = stock_notification;
        }

        public Double getCost_per_item() {
            return cost_per_item;
        }

        public void setCost_per_item(Double cost_per_item) {
            this.cost_per_item = cost_per_item;
        }

        public Double getCompare_at_price() {
            return compare_at_price;
        }

        public void setCompare_at_price(Double compare_at_price) {
            this.compare_at_price = compare_at_price;
        }

        public int getMinimum_quantity() {
            return minimum_quantity;
        }

        public void setMinimum_quantity(int minimum_quantity) {
            this.minimum_quantity = minimum_quantity;
        }

        public int getMaximum_quantity() {
            return maximum_quantity;
        }

        public void setMaximum_quantity(int maximum_quantity) {
            this.maximum_quantity = maximum_quantity;
        }

        public String getSku() {
            return sku;
        }

        public void setSku(String sku) {
            this.sku = sku;
        }

        public String getBrand() {
            return brand;
        }

        public void setBrand(String brand) {
            this.brand = brand;
        }

        public String getBarcode() {
            return barcode;
        }

        public void setBarcode(String barcode) {
            this.barcode = barcode;
        }

        public String getGoogle_product_category() {
            return google_product_category;
        }

        public void setGoogle_product_category(String google_product_category) {
            this.google_product_category = google_product_category;
        }

        public boolean isFeatured() {
            return featured;
        }

        public void setFeatured(boolean featured) {
            this.featured = featured;
        }

        public boolean isShipping_required() {
            return shipping_required;
        }

        public void setShipping_required(boolean shipping_required) {
            this.shipping_required = shipping_required;
        }

        public boolean isReviews_enabled() {
            return reviews_enabled;
        }

        public void setReviews_enabled(boolean reviews_enabled) {
            this.reviews_enabled = reviews_enabled;
        }

        public String getStatus() {
            return status;
        }

        public void setStatus(String status) {
            this.status = status;
        }

        public String getCreated_at() {
            return created_at;
        }

        public void setCreated_at(String created_at) {
            this.created_at = created_at;
        }

        public String getUpdated_at() {
            return updated_at;
        }

        public void setUpdated_at(String updated_at) {
            this.updated_at = updated_at;
        }

        public String getPackage_format() {
            return package_format;
        }

        public void setPackage_format(String package_format) {
            this.package_format = package_format;
        }

        public double getLength() {
            return length;
        }

        public void setLength(double length) {
            this.length = length;
        }

        public double getWidth() {
            return width;
        }

        public void setWidth(double width) {
            this.width = width;
        }

        public double getHeight() {
            return height;
        }

        public void setHeight(double height) {
            this.height = height;
        }

        public double getDiameter() {
            return diameter;
        }

        public void setDiameter(double diameter) {
            this.diameter = diameter;
        }

        public String getPermalink() {
            return permalink;
        }

        public void setPermalink(String permalink) {
            this.permalink = permalink;
        }

        public Category[] getCategories() {
            return categories;
        }

        public void setCategories(Category[] categories) {
            this.categories = categories;
        }
    }
}