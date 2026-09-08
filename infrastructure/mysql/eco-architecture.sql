CREATE TABLE IF NOT EXISTS ecom_api_user (
	id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	username VARCHAR(100) NOT NULL,
	password_hash VARCHAR(255) NOT NULL,
	role VARCHAR(50) NOT NULL DEFAULT 'api_client',
	is_active TINYINT(1) NOT NULL DEFAULT 1,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	UNIQUE KEY uq_ecom_api_user_username (username)
);

CREATE TABLE IF NOT EXISTS ecom_api_token (
	id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
	jti CHAR(32) NOT NULL,
	user_id BIGINT UNSIGNED NOT NULL,
	token TEXT NOT NULL,
	issued_at DATETIME NOT NULL,
	expires_at DATETIME NOT NULL,
	revoked_at DATETIME NULL,
	user_agent VARCHAR(255) NULL,
	client_ip VARCHAR(64) NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	UNIQUE KEY uq_ecom_api_token_jti (jti),
	KEY idx_ecom_api_token_user (user_id),
	KEY idx_ecom_api_token_expiration (expires_at),
	CONSTRAINT fk_ecom_api_token_user
	  FOREIGN KEY (user_id) REFERENCES ecom_api_user(id)
	  ON DELETE CASCADE
);

-- Usuario inicial (reemplazar password_hash por un hash bcrypt real).
INSERT INTO ecom_api_user (username, password_hash, role)
VALUES ('admin', '$2a$10$REPLACE_WITH_BCRYPT_HASH', 'admin')
ON DUPLICATE KEY UPDATE username = username;

create table ecom_sources (
    id bigint unsigned primary key auto_increment,
    code varchar(50) not null ,
    name varchar(500) not null ,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_source_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_source_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_storage_disks (
    id bigint unsigned primary key auto_increment,
    name varchar(100) not null ,
    code varchar(50) not null unique ,
    base_url varchar(500) not null ,
    bucket varchar(255) default null ,
    endpoint varchar(500) default null,
    is_public boolean default 1,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null
);

create table ecom_files (
    id bigint unsigned primary key auto_increment,
    disk_id bigint unsigned not null ,
    path varchar(500) not null ,
    filename varchar(255) not null ,
    original_filename varchar(255) default null,
    mime_type varchar(100) not null ,
    file_type enum('image', 'document', 'video', 'icon', 'archive', 'other') default 'image',
    extension varchar(20) not null ,
    size bigint unsigned not null ,
    checksum char(64) default null,
    width int unsigned default null,
    height int unsigned default null,
    is_public boolean not null default 1,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_file_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_file_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_brands (
     id bigint unsigned primary key auto_increment,
     name varchar(255) not null ,
     created_by bigint unsigned not null ,
     updated_by bigint unsigned default null,
     created_at timestamp default current_timestamp,
     updated_at timestamp default current_timestamp on update current_timestamp,
     deleted_at timestamp default null,
     constraint fk_brand_created_by foreign key (created_by) references ecom_api_user(id),
     constraint fk_brand_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_categories (
     id bigint unsigned primary key auto_increment,
     name varchar(255) not null ,
     parent_id bigint unsigned default null,
     created_by bigint unsigned not null ,
     updated_by bigint unsigned default null,
     created_at timestamp default current_timestamp,
     updated_at timestamp default current_timestamp on update current_timestamp,
     deleted_at timestamp default null,
     constraint fk_parent_category foreign key (parent_id) references ecom_categories(id),
     constraint fk_category_created_by foreign key (created_by) references ecom_api_user(id),
     constraint fk_category_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_products (
    id bigint unsigned primary key auto_increment,
    sku varchar(255) not null unique ,
    part_number varchar(255) not null ,
    name varchar(255) not null ,
    description text default null,
    short_description varchar(255) default null,
    brand_id bigint unsigned default null ,
    category_id bigint unsigned default null ,
    product_type enum('part', 'consumable', 'accessory') default 'part',
    status enum('active', 'discontinued', 'hidden'),
    is_sellable boolean default 1,
    is_stockable boolean default 1,
    source_id bigint unsigned not null,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_product_brand foreign key (brand_id) references ecom_brands(id),
    constraint fk_product_category foreign key (category_id) references ecom_categories(id),
    constraint fk_product_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_product_updated_by foreign key (updated_by) references ecom_api_user(id),
    constraint fk_product_source foreign key (source_id) references ecom_sources(id),
    -- sku es el único identificador realmente único del producto (unique arriba, global,
    -- sin importar el source_id). part_number NO es único: dos productos del mismo source_id
    -- pueden compartir part_number (ver FindBySourceAndPartNumber /
    -- FindAllBySourceAndPartNumber en product_repository.go, que ya no asumen un único
    -- resultado). Este índice solo sostiene fk_product_source — antes ese rol lo cumplía
    -- uq_products_source_part_number, eliminado junto con la unicidad de part_number.
    index idx_products_source_id (source_id)
);

create table ecom_product_part_numbers (
    product_id bigint unsigned not null ,
    part_number varchar(255) not null ,
    type enum('aftermarket', 'cross_reference', 'supplier', 'internal') default 'cross_reference',
    brand_id bigint unsigned not null ,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_product_part_number foreign key (product_id) references ecom_products(id),
    constraint fk_part_number_brand foreign key (brand_id) references ecom_brands(id),
    constraint fk_part_number_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_part_number_updated_by foreign key (updated_by) references ecom_api_user(id)
);

-- Cadena de sucesion de numeros de parte: cuando el ERP marca (PROD_SUPERSESION) que
-- old_part_number fue reemplazado por new_part_number. Ninguno de los dos lados tiene
-- garantizado existir todavia en ecom_products (mismo problema que
-- ecom_pending_product_vehicle_fitments, pero aqui ambos lados pueden estar pendientes,
-- no solo uno), por eso old_product_id/new_product_id son nullable y se resuelven de forma
-- independiente cuando ProductService crea el producto correspondiente.
create table ecom_part_number_supersessions (
    id bigint unsigned primary key auto_increment,
    source_id bigint unsigned not null,
    old_part_number varchar(255) not null,
    new_part_number varchar(255) not null,
    old_product_id bigint unsigned default null,
    new_product_id bigint unsigned default null,
    old_resolved_at timestamp default null,
    new_resolved_at timestamp default null,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_pns_source foreign key (source_id) references ecom_sources(id),
    constraint fk_pns_old_product foreign key (old_product_id) references ecom_products(id),
    constraint fk_pns_new_product foreign key (new_product_id) references ecom_products(id),
    constraint fk_pns_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_pns_updated_by foreign key (updated_by) references ecom_api_user(id),
    -- Estado actual, no log: a lo mas una sucesion vigente por pieza dentro de la misma
    -- fuente (igual que ecom_product_stock/ecom_product_prices). Si el ERP corrige a que
    -- pieza apunta PROD_SUPERSESION, se actualiza esta misma fila (ver Update en
    -- part_number_supersessions_repository.go) en vez de insertar una nueva.
    unique key uq_pns_old_part_number (source_id, old_part_number),
    index idx_pns_new (source_id, new_part_number)
);

create table ecom_product_dimensions (
    product_id bigint unsigned not null ,
    weight decimal(10,2) default 1.00,
    length decimal(10,2) default 1.00,
    width decimal(10,2) default 1.00,
    height decimal(10,2) default 1.00,
    diameter decimal(10,2) default 1.00,
    volume decimal(10,2) default 1.00,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_product_dimensions_id foreign key (product_id) references ecom_products(id),
    constraint fk_dimensions_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_dimensions_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_product_seo (
    product_id bigint unsigned not null ,
    meta_title varchar(255) default null,
    meta_description varchar(255) default null,
    keywords text default null,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default  null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_product_seo_id foreign key (product_id) references ecom_products(id),
    constraint fk_seo_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_seo_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_product_media (
    product_id bigint unsigned not null ,
    type enum('image', 'manual', 'datasheet', 'certificate') default 'image',
    file_id bigint unsigned not null ,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_product_media_id foreign key (product_id) references ecom_products(id),
    constraint fk_product_media_file foreign key (file_id) references ecom_files(id) on delete cascade ,
    constraint fk_media_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_media_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_branches (
    id bigint unsigned primary key auto_increment,
    name varchar(255) not null ,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_branch_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_branch_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_warehouses (
    id bigint unsigned primary key auto_increment,
    name varchar(255) not null ,
    branch_id bigint unsigned not null ,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_warehouse_branch foreign key (branch_id) references ecom_branches(id),
    constraint fk_warehouse_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_warehouse_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_product_stock (
    id bigint unsigned primary key auto_increment,
    product_id bigint unsigned not null ,
    branch_id bigint unsigned not null ,
    warehouse_id bigint unsigned not null ,
    available_qty int default 0,
    last_sync_at timestamp default null,
    updated_by bigint unsigned default null,
    constraint fk_product_stock_id foreign key (product_id) references ecom_products(id),
    constraint fk_stock_branch foreign key (branch_id) references ecom_branches(id),
    constraint fk_stock_warehouse foreign key (warehouse_id) references ecom_warehouses(id),
    constraint fk_stock_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_stock_movements (
    id bigint unsigned primary key auto_increment,
    product_id bigint unsigned not null ,
    branch_id bigint unsigned not null ,
    warehouse_id bigint unsigned not null ,
    movement_type enum('in', 'out', 'adjustment', 'transfer_in', 'transfer_out', 'sync') default 'adjustment',
    quantity_before int,
    quantity_change int,
    quantity_after int,
    updated_by bigint unsigned not null ,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_stock_movement_product foreign key (product_id) references ecom_products(id),
    constraint fk_stock_movement_branch foreign key (branch_id) references ecom_branches(id),
    constraint fk_stock_movement_warehouse foreign key (warehouse_id) references ecom_warehouses(id),
    constraint fk_stock_move_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_currencies (
    id bigint unsigned primary key auto_increment,
    name varchar(255) not null ,
    code varchar(255) not null unique ,
    symbol char(10) not null ,
    decimal_places int default 2,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_currency_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_currency_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_exchange_rates (
    id bigint unsigned primary key auto_increment,
    from_currency_id bigint unsigned not null ,
    to_currency_id bigint unsigned not null ,
    rate decimal(10,2) default 1.00,
    updated_by bigint unsigned not null ,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_currency_from foreign key (from_currency_id) references ecom_currencies(id),
    constraint fk_currency_to foreign key (to_currency_id) references ecom_currencies(id),
    constraint fk_curr_rate_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_price_list (
    id bigint unsigned primary key auto_increment,
    name varchar(255) not null ,
    currency bigint unsigned not null ,
    priority int default 0,
    status enum('active', 'discontinued', 'hidden') default 'active',
    valid_from date not null ,
    valid_to date not null,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_price_list_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_price_list_updated_by foreign key (updated_by) references ecom_api_user(id)
);

-- Formula de precios que convierte el precio base (ecom_product_prices, vía
-- FindEffectivePrice) en el precio publicado en marketplaces (MercadoLibre,
-- Odoo). expression es una fórmula matemática en texto que solo puede
-- referenciar la variable "base" (ej. "(((base*1.13)/0.85)+90)*1.16"),
-- evaluada en Go con github.com/expr-lang/expr. brand_id, connection_id y/o
-- price_list_id null = comodín ("cualquier marca" / "cualquier canal" /
-- "cualquier lista de precios"); la fila con los tres null es el default
-- universal. price_list_id referencia la lista que ganó al resolver el
-- precio efectivo del producto (ecom_price_list, vía FindEffectivePrice),
-- no una lista elegida a mano. PricingFormulaRepository.Resolve elige la
-- fila más específica disponible, en orden de prioridad: connection_id >
-- brand_id > price_list_id, con más dimensiones coincidentes ganando ante
-- menos (ver el comentario de Resolve para la tabla completa de 8 combinaciones).
-- MySQL permite múltiples NULL en una unique key (y los trata como valores
-- distintos incluso dentro de una key compuesta), así que la unicidad de
-- cada combinación (brand_id, connection_id, price_list_id) se refuerza en
-- la capa de aplicación, no aquí. idx_pricing_formula_brand sostiene
-- fk_pricing_formula_brand.
create table ecom_pricing_formulas (
    id bigint unsigned primary key auto_increment,
    brand_id bigint unsigned default null,
    connection_id bigint unsigned default null,
    price_list_id bigint unsigned default null,
    expression varchar(1000) not null,
    description varchar(255) default null,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_pricing_formula_brand foreign key (brand_id) references ecom_brands(id),
    constraint fk_pricing_formula_connection foreign key (connection_id) references ecom_channel_connections(id),
    constraint fk_pricing_formula_price_list foreign key (price_list_id) references ecom_price_list(id),
    constraint fk_pricing_formula_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_pricing_formula_updated_by foreign key (updated_by) references ecom_api_user(id),
    index idx_pricing_formula_brand (brand_id)
);

create table ecom_product_prices (
    id bigint unsigned primary key auto_increment,
    product_id bigint unsigned not null ,
    price_list_id bigint unsigned not null ,
    price decimal(10,2) default 0.00,
    currency bigint unsigned not null ,
    margin decimal(10,2) default 0.00,
    tax_included  boolean default 0,
    updated_by bigint unsigned not null ,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    constraint fk_product_price_id foreign key (product_id) references ecom_products(id),
    constraint fk_product_price_list foreign key (price_list_id) references ecom_price_list(id),
    constraint fk_price_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_price_history (
    id bigint unsigned primary key auto_increment,
    product_id bigint unsigned not null ,
    price_list_id bigint unsigned not null ,
    currency_id bigint unsigned not null ,
    old_price decimal(10,2) not null ,
    new_price decimal(10,2) not null ,
    updated_by bigint unsigned not null ,
    created_at timestamp default current_timestamp,
    constraint fk_price_history_product foreign key (product_id) references ecom_products(id),
    constraint fk_price_history_list foreign key (price_list_id) references ecom_price_list(id),
    constraint fk_price_history_currency foreign key (currency_id) references ecom_currencies(id),
    constraint fk_price_history_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_product_images (
     id bigint unsigned primary key auto_increment,
     product_id bigint unsigned not null ,
     file_id bigint unsigned not null ,
     is_first boolean default 0,
     created_by bigint unsigned not null ,
     updated_by bigint unsigned default null,
     created_at timestamp default current_timestamp,
     updated_at timestamp default current_timestamp on update current_timestamp,
     deleted_at timestamp default null,
     constraint fk_product_image_id foreign key (product_id) references ecom_products(id),
     constraint fk_product_image_created foreign key (created_by) references ecom_api_user(id),
     constraint fk_product_image_updated foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_product_videos (
    id bigint unsigned primary key auto_increment,
    product_id bigint unsigned not null ,
    file_id bigint unsigned not null ,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_product_video_id foreign key (product_id) references ecom_products(id),
    constraint fk_product_video_file_id foreign key (file_id) references ecom_files(id),
    constraint fk_product_video_created foreign key (created_by) references ecom_api_user(id),
    constraint fk_product_video_updated foreign key (updated_by) references ecom_api_user(id)
);

/* ================================================================================================================ */

create table ecom_channels (
    id bigint unsigned primary key auto_increment,
    name varchar(255) not null ,
    code varchar(255) not null unique ,
    status enum('active', 'discontinued', 'hidden') default 'active',
    icon_id bigint unsigned default null,
    description text default null,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_channel_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_channel_updated_by foreign key (updated_by) references ecom_api_user(id),
    constraint fk_channel_icon foreign key (icon_id) references ecom_files(id)
);

create table ecom_channel_connections (
    id bigint unsigned primary key auto_increment,
    channel_id bigint unsigned not null ,
    name varchar(255) not null ,
    status enum('active', 'discontinued', 'hidden') default 'active',
    environment enum('development', 'production') default 'development',
    currency_id bigint unsigned not null ,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    allows_multiple_listings boolean not null default 0,
    constraint fk_channel_connection_id foreign key (channel_id) references ecom_channels(id),
    constraint fk_channel_currency foreign key (currency_id) references ecom_currencies(id),
    constraint fk_channel_conn_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_channel_conn_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_connection_credentials (
    id bigint unsigned primary key auto_increment,
    connection_id bigint unsigned not null ,
    key_name varchar(255) not null ,
    value text not null ,
    is_encrypted boolean default 0,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_conn_credential_id foreign key (connection_id) references ecom_channel_connections(id),
    constraint fk_conn_credential_created foreign key (created_by) references ecom_api_user(id),
    constraint fk_conn_credential_updated foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_connection_settings (
    id bigint unsigned primary key auto_increment,
    connection_id bigint unsigned not null ,
    key_name varchar(255) not null,
    value text not null ,
    is_encrypted boolean default 0,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_conn_settings_id foreign key (connection_id) references ecom_channel_connections(id),
    constraint fk_conn_settings_created foreign key (created_by) references ecom_api_user(id),
    constraint fk_conn_settings_updated foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_connection_status (
    id bigint unsigned primary key auto_increment,
    connection_id bigint unsigned not null ,
    authenticated boolean default 0,
    last_auth timestamp default null,
    last_error text default null,
    expires_at timestamp default null,
    last_sync timestamp default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    constraint fk_conn_status_id foreign key (connection_id) references ecom_channel_connections(id)
);

create table ecom_channel_parameters (
    id bigint unsigned primary key auto_increment,
    channel_id bigint unsigned not null ,
    parameter_name varchar(255) not null ,
    display_name varchar(255) not null ,
    parameter_type enum('text', 'number', 'boolean') default 'text',
    required boolean default 1,
    default_value varchar(255) default null,
    is_encrypted boolean default 0,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_channel_param_id foreign key (channel_id) references ecom_channels(id),
    constraint fk_channel_param_created foreign key (created_by) references ecom_api_user(id),
    constraint fk_channel_param_updated foreign key (updated_by) references ecom_api_user(id)
);

/* ============================ Product sync =============== */

create table ecom_channel_category_map (
    id bigint unsigned primary key auto_increment,
    category_id bigint unsigned not null ,
    connection_id bigint unsigned not null ,
    external_category_id varchar(255) not null,
    external_category_name varchar(255) default null,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    unique key uq_category_connection (category_id, connection_id),
    constraint fk_cat_map_category foreign key (category_id) references ecom_categories(id),
    constraint fk_cat_map_connection foreign key (connection_id) references ecom_channel_connections(id),
    constraint fk_cat_map_created foreign key (created_by) references ecom_api_user(id),
    constraint fk_cat_map_updated foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_channel_product_map (
    id bigint unsigned primary key auto_increment,
    listing_title varchar(255) default null,
    product_id bigint unsigned not null ,
    connection_id bigint unsigned not null ,
    vehicle_fitment_id bigint unsigned default null,
    vehicle_fitment_key bigint unsigned generated always as (coalesce(vehicle_fitment_id, 0)) stored,
    is_enabled boolean not null default 1,
    external_id varchar(100) default null,
    external_category_id varchar(255) default null,
    status enum('pending', 'synced', 'error', 'under_review', 'paused', 'closed') not null default 'pending',
    last_synced_at timestamp default null,
    last_error text default null,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    unique key uq_product_connection_fitment (product_id, connection_id, vehicle_fitment_key),
    constraint fk_prod_map_product foreign key (product_id) references ecom_products(id),
    constraint fk_prod_map_connection foreign key (connection_id) references ecom_channel_connections(id),
    constraint fk_prod_map_fitment foreign key (vehicle_fitment_id) references ecom_vehicle_fitments(id),
    constraint fk_prod_map_created foreign key (created_by) references ecom_api_user(id),
    constraint fk_prod_map_updated foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_channel_sync_queue (
    id bigint unsigned primary key auto_increment,
    product_id bigint unsigned not null ,
    connection_id bigint unsigned not null ,
    sync_type enum('price', 'stock', 'full', 'listing') not null  default 'full',
    status enum('pending', 'processing', 'done', 'failed') not null default 'pending',
    attempts int unsigned not null default 0,
    last_error text default null,
    requested_at timestamp default current_timestamp,
    processed_at timestamp default null,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_sync_queue_product foreign key (product_id) references ecom_products(id),
    constraint fk_sync_queue_connection foreign key (connection_id) references ecom_channel_connections(id),
    constraint fk_sync_queue_updated foreign key (updated_by) references ecom_api_user(id),
    index idx_sync_queue_pending (status, connection_id)
);

/* ============ Mercado Libre notifications (/meli_notifications) ==================================== */

-- Raw inbox for MercadoLibre webhook notifications (POST /meli_notifications,
-- registered as the app's notification callback in MELI's developer
-- console). Each row is one notification delivery, stored as-is and marked
-- 'pending' for whatever process consumes it next: MercadoLibre notification
-- bodies never carry the changed data itself, only a pointer (`resource`) to
-- re-fetch with the owning connection's access token, so this table is
-- intentionally a durable inbox/log, not a projection of business data.
-- meli_user_id/application_id/delivery_attempts/meli_sent_at/meli_received_at
-- are copied verbatim from MercadoLibre's payload; ingested_at is stamped by
-- us on arrival. connection_id is left nullable because resolving which
-- ecom_channel_connections row owns a given meli_user_id is deferred to
-- whatever processes the queue, not done synchronously on ingestion.
create table ecom_meli_notifications (
    id bigint unsigned primary key auto_increment,
    notification_id varchar(100) not null,
    resource varchar(255) not null,
    topic varchar(50) not null,
    meli_user_id bigint unsigned not null,
    application_id bigint unsigned not null,
    delivery_attempts int unsigned not null default 1,
    meli_sent_at timestamp default null,
    meli_received_at timestamp default null,
    ingested_at timestamp not null default current_timestamp,
    connection_id bigint unsigned default null,
    raw_payload text not null,
    status enum('pending', 'processing', 'done', 'failed', 'ignored') not null default 'pending',
    last_error text default null,
    processed_at timestamp default null,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    unique key uq_meli_notification_id (notification_id),
    index idx_meli_notifications_pending (status, topic),
    index idx_meli_notifications_user (meli_user_id),
    constraint fk_meli_notification_connection foreign key (connection_id) references ecom_channel_connections(id),
    constraint fk_meli_notification_updated_by foreign key (updated_by) references ecom_api_user(id)
);

/* ============ Compatibilities ============================0 */

create table ecom_equipment_types (
    id bigint unsigned primary key auto_increment,
    name varchar(255) not null,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_equipment_type_created foreign key (created_by) references ecom_api_user(id),
    constraint fk_equipment_type_updated foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_vehicle_fitments (
    id bigint unsigned primary key auto_increment,
    brand_id bigint unsigned not null ,
    model varchar(255) not null ,
    year_start smallint unsigned not null ,
    year_end smallint unsigned default null,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_vehicle_fit_brand foreign key (brand_id) references ecom_brands(id),
    constraint fk_vehicle_fit_created foreign key (created_by) references ecom_api_user(id),
    constraint fk_vehicle_fit_updated foreign key (updated_by) references ecom_api_user(id),
    unique key uq_vehicle_fitment (brand_id, model, year_start, year_end)
);

create table ecom_equipment_fitment (
    id bigint unsigned primary key auto_increment,
    brand_id bigint unsigned not null ,
    equipment_type bigint unsigned not null ,
    model varchar(255) default null,
    serie varchar(255) default null,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_equipment_fit_brand foreign key (brand_id) references ecom_brands(id),
    constraint fk_equipment_fit_type foreign key (equipment_type) references ecom_equipment_types(id),
    constraint fk_equipment_fit_created foreign key (created_by) references ecom_api_user(id),
    constraint fk_equipment_fit_updated foreign key (updated_by) references ecom_api_user(id),
    constraint chk_equipment_model_or_serie check (model is not null or serie is not null)
);

create table ecom_product_vehicle_compatibility (
    id bigint unsigned primary key auto_increment,
    product_id bigint unsigned not null ,
    vehicle_fitment_id bigint unsigned  not null ,
    motor varchar(100) not null default '',       -- MercadoLibre VIS qualifier: how this product occupies the fitment, not a vehicle property
    position varchar(100) not null default '',
    side varchar(100) not null default '',
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_pvc_product foreign key (product_id) references ecom_products(id),
    constraint fk_pvc_fitment foreign key (vehicle_fitment_id) references ecom_vehicle_fitments(id),
    constraint fk_pvc_created foreign key (created_by) references ecom_api_user(id),
    constraint fk_pvc_updated foreign key (updated_by) references ecom_api_user(id),
    unique key uq_product_vehicle_fitment (product_id, vehicle_fitment_id, motor, position, side)
);

create table ecom_product_equipment_compatibility (
    id bigint unsigned primary key auto_increment,
    product_id bigint unsigned not null,
    equipment_fitment_id bigint unsigned not null,
    created_by bigint unsigned not null,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_pec_product foreign key (product_id) references ecom_products(id),
    constraint fk_pec_fitment foreign key (equipment_fitment_id) references ecom_equipment_fitment(id),
    constraint fk_pec_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_pec_updated_by foreign key (updated_by) references ecom_api_user(id),
    unique key uq_product_equipment_fitment (product_id, equipment_fitment_id)
);

create table ecom_temporal_uploads (
    id bigint unsigned primary key auto_increment,
    product_id bigint unsigned,
    external_id varchar(255) not null ,
    title text default null,
    stock integer default 0,
    price decimal(10,2) default '1.00',
    external_category_id varchar(255) default null,
    status varchar(255) default null,
    created_at timestamp default current_timestamp,
    permalink text default null
);

-- Staging table for product-vehicle compatibilities uploaded (e.g. via bulk
-- import) with a SKU that doesn't exist in ecom_products yet. Once a product
-- with a matching SKU is created, ProductService resolves these rows into
-- real ecom_product_vehicle_compatibility records and stamps resolved_at/
-- resolved_product_id here for traceability instead of deleting them.
create table ecom_pending_product_vehicle_fitments (
    id bigint unsigned primary key auto_increment,
    sku varchar(255) not null ,
    vehicle_fitment_id bigint unsigned not null ,
    motor varchar(100) not null default '',
    position varchar(100) not null default '',
    side varchar(100) not null default '',
    resolved_at timestamp default null,
    resolved_product_id bigint unsigned default null,
    created_by bigint unsigned not null ,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_ppvf_fitment foreign key (vehicle_fitment_id) references ecom_vehicle_fitments(id),
    constraint fk_ppvf_resolved_product foreign key (resolved_product_id) references ecom_products(id),
    constraint fk_ppvf_created foreign key (created_by) references ecom_api_user(id),
    constraint fk_ppvf_updated foreign key (updated_by) references ecom_api_user(id),
    unique key uq_pending_sku_fitment (sku, vehicle_fitment_id, motor, position, side)
);

/* ============ ATTRIBUTES ==================================== */

create table ecom_attributes (
    id bigint unsigned primary key auto_increment,
    code varchar(100) not null unique,          -- 'COLOR', 'VOLTAGE', 'MATERIAL'...
    name varchar(255) not null,                  -- "Color", "Voltaje"
    data_type enum('text','number','boolean','date','enum') default 'text',
    unit varchar(50) default null,                -- 'kg', 'cm', 'V'
    created_by bigint unsigned not null,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_attribute_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_attribute_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_attribute_options (
    id bigint unsigned primary key auto_increment,
    attribute_id bigint unsigned not null,
    value varchar(255) not null,
    external_value_id varchar(255) default null,
    created_by bigint unsigned not null,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    constraint fk_attr_option_attribute foreign key (attribute_id) references ecom_attributes(id),
    constraint fk_attr_option_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_attr_option_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_product_attributes (
    id bigint unsigned primary key auto_increment,
    product_id bigint unsigned not null,
    attribute_id bigint unsigned not null,
    value_text varchar(500) default null,
    value_number decimal(15,4) default null,
    value_boolean boolean default null,
    value_date date default null,
    option_id bigint unsigned default null,   -- cuando data_type = 'enum'
    created_by bigint unsigned not null,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    unique key uq_product_attribute (product_id, attribute_id),
    constraint fk_product_attr_product foreign key (product_id) references ecom_products(id),
    constraint fk_product_attr_attribute foreign key (attribute_id) references ecom_attributes(id),
    constraint fk_product_attr_option foreign key (option_id) references ecom_attribute_options(id),
    constraint fk_product_attr_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_product_attr_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_channel_attributes (
    id bigint unsigned primary key auto_increment,
    channel_id bigint unsigned not null,
    target_strategy enum('fixed_key','dynamic_field') not null default 'fixed_key',
    -- fixed_key: el canal tiene una llave fija conocida (ML: BRAND, PART_NUMBER, MODEL...)
    -- dynamic_field: el canal no tiene llaves fijas, el valor se inserta en un campo/mecanismo del modelo (Odoo)
    external_key varchar(100) default null,      -- obligatorio si target_strategy = 'fixed_key'
    target_field varchar(100) default null,       -- obligatorio si target_strategy = 'dynamic_field' (p.ej. 'attribute_line_ids', 'description_ecommerce')
    external_label varchar(255) default null,
    value_mode enum('value_name','value_id') default 'value_name', -- ML: algunos atributos van por value_id (ITEM_CONDITION)
    category_id bigint unsigned default null,      -- null = aplica a todas las categorías; distinto de null = solo esa categoría local (atributos que ML exige por categoría)
    is_required boolean default 0,
    created_by bigint unsigned not null,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    unique key uq_channel_attribute (channel_id, external_key, category_id),
    constraint fk_channel_attr_channel foreign key (channel_id) references ecom_channels(id),
    constraint fk_channel_attr_category foreign key (category_id) references ecom_categories(id),
    constraint fk_channel_attr_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_channel_attr_updated_by foreign key (updated_by) references ecom_api_user(id)
);

create table ecom_channel_attribute_map (
    id bigint unsigned primary key auto_increment,
    channel_attribute_id bigint unsigned not null,
    connection_id bigint unsigned default null,   -- null = aplica a toda conexión de ese canal; distinto de null = override puntual
    source_type enum('custom_attribute','system_field','static_value') not null,
    attribute_id bigint unsigned default null,     -- cuando source_type = 'custom_attribute'
    system_field varchar(100) default null,          -- cuando source_type = 'system_field' (whitelist en código: 'brand.name','part_number','sku','dimensions.weight'...)
    static_value varchar(255) default null,          -- cuando source_type = 'static_value'
    created_by bigint unsigned not null,
    updated_by bigint unsigned default null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp on update current_timestamp,
    deleted_at timestamp default null,
    unique key uq_channel_attribute_map (channel_attribute_id, connection_id),
    constraint fk_channel_attr_map_slot foreign key (channel_attribute_id) references ecom_channel_attributes(id),
    constraint fk_channel_attr_map_connection foreign key (connection_id) references ecom_channel_connections(id),
    constraint fk_channel_attr_map_attribute foreign key (attribute_id) references ecom_attributes(id),
    constraint fk_channel_attr_map_created_by foreign key (created_by) references ecom_api_user(id),
    constraint fk_channel_attr_map_updated_by foreign key (updated_by) references ecom_api_user(id)
);