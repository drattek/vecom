package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

// jwtSecretPlaceholder es el valor que traía el fallback anterior. Se rechaza
// explícitamente para que nadie lo copie al .env pensando que "ya está seteado".
const jwtSecretPlaceholder = "change-me-in-production"

type Config struct {
	// Port es el puerto HTTPS de core-orchestrator. Siempre 443 (env SERVER_PORT,
	// default 443) — coincide con docker-compose.yml y .env.
	Port string

	// ShutdownTimeout acota cuánto espera main() a que server.Shutdown drene
	// las requests en vuelo (crear listing, actualizar precio, etc.) ante un
	// SIGTERM/SIGINT antes de forzar el cierre. Debe ser menor que el período
	// de gracia que el orquestador de deploy concede al contenedor.
	ShutdownTimeout time.Duration

	TLSKeystorePath     string
	TLSKeystorePassword string
	TLSKeystoreType     string

	JWTSecret     string
	JWTIssuer     string
	JWTTTLMinutes int

	MySQLHost     string
	MySQLPort     string
	MySQLUser     string
	MySQLPassword string
	MySQLDatabase string

	// Pool de conexiones MySQL (database/sql). Sin límites, un pico de carga o
	// el endpoint de migración disparando muchas queries puede agotar
	// max_connections de MySQL; sin ConnMaxLifetime aparecen "invalid
	// connection" intermitentes tras un reinicio de MySQL o detrás de un LB.
	// Si MySQLMaxIdleConns supera a MySQLMaxOpenConns, database/sql lo recorta
	// solo al valor de open.
	MySQLMaxOpenConns    int
	MySQLMaxIdleConns    int
	MySQLConnMaxLifetime time.Duration
	MySQLConnMaxIdleTime time.Duration

	RabbitMQHost     string
	RabbitMQPort     string
	RabbitMQUser     string
	RabbitMQPassword string

	RedisHost string
	RedisPort string

	// TokenRefreshPollInterval and TokenRefreshLookahead configure the
	// (not-yet-started) marketplace token refresh scheduler. See
	// internal/interfaces/schedulers/token_refresh_scheduler.go.
	TokenRefreshPollInterval time.Duration
	TokenRefreshLookahead    time.Duration

	// SyncQueuePollInterval and SyncQueueBatchSize configure the marketplace
	// consumer (internal/workers/marketplace_worker.go), which polls
	// ecom_channel_sync_queue for pending 'listing' rows and publishes them.
	// Default poll interval is 30 min — the rows are produced once a day by
	// ListingDiscoveryScheduler, so there is nothing to gain from polling
	// tighter, and publishing hits rate-limited marketplace APIs.
	SyncQueuePollInterval time.Duration
	SyncQueueBatchSize    int

	// ListingDiscoveryRunAtHour/ListingDiscoveryRunAtMinute configure the
	// server-local time of day ListingDiscoveryScheduler runs its once-a-day
	// scan for products ready to publish (see ADR 0003 and
	// internal/interfaces/schedulers/listing_discovery_scheduler.go). Default
	// is 01:00.
	ListingDiscoveryRunAtHour   int
	ListingDiscoveryRunAtMinute int

	// CompatibilitiesFixRunAtHour/CompatibilitiesFixRunAtMinute configure the
	// server-local time of day CompatibilitiesFixScheduler runs once a day,
	// re-checking every active MERCADOLIBRE connection for listings still
	// needing vehicle compatibilities pushed (see
	// internal/interfaces/schedulers/compatibilities_fix_scheduler.go). Each
	// run costs ~2 MercadoLibre API calls per eligible row (GetItem +
	// create), against a 10-calls/minute budget shared by the whole app —
	// FixUnderReviewListings processes every eligible row per connection, so
	// a large backlog makes the run take longer rather than fail.
	CompatibilitiesFixRunAtHour   int
	CompatibilitiesFixRunAtMinute int

	// SIEAPIToken authenticates requests to Banco de México's SIE API (see
	// internal/infrastructure/banxico), used by ExchangeRateScheduler to
	// keep ecom_exchange_rates' USD→MXN row current daily.
	SIEAPIToken string

	// ExchangeRateRunAtHour/ExchangeRateRunAtMinute configure the
	// server-local time of day ExchangeRateScheduler runs once a day. See
	// internal/interfaces/schedulers/exchange_rate_scheduler.go — default
	// is 08:00 which, on the UTC deployment server, is 02:00 CDMX (UTC-6);
	// the job then picks up the most recent FIX rate already published by
	// Banxico (SF43718, published ~12:00-13:00 hrs CDMX the prior business
	// day).
	ExchangeRateRunAtHour   int
	ExchangeRateRunAtMinute int
}

func Load() Config {
	// core-orchestrator siempre sirve HTTPS en :443 (ver docker-compose.yml y
	// .env). El default acá coincide con eso para que el binario levante en el
	// puerto correcto aunque falte SERVER_PORT.
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "443"
	}

	tlsKeystorePath := os.Getenv("SERVER_TLS_KEYSTORE_PATH")
	tlsKeystorePassword := os.Getenv("SERVER_TLS_KEYSTORE_PASSWORD")
	tlsKeystoreType := os.Getenv("SERVER_TLS_KEYSTORE_TYPE")
	if tlsKeystoreType == "" {
		tlsKeystoreType = "PKCS12"
	}

	// JWT_SECRET es obligatorio: sin él (o con el placeholder viejo) cualquiera
	// puede firmar un JWT válido —rol admin incluido— y RequireAuth lo acepta.
	// Se aborta el arranque en vez de caer a un secreto público.
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" || jwtSecret == jwtSecretPlaceholder {
		log.Fatal("JWT_SECRET no está definido (o usa el valor placeholder): la app no puede arrancar con un secreto de firma público")
	}

	jwtIssuer := os.Getenv("JWT_ISSUER")
	if jwtIssuer == "" {
		jwtIssuer = "core-orchestrator"
	}

	jwtTTLMinutes := 60
	if rawTTL := os.Getenv("JWT_TTL_MINUTES"); rawTTL != "" {
		if parsedTTL, err := strconv.Atoi(rawTTL); err == nil && parsedTTL > 0 {
			jwtTTLMinutes = parsedTTL
		}
	}

	mysqlHost := os.Getenv("MYSQL_HOST")
	if mysqlHost == "" {
		mysqlHost = "localhost"
	}

	mysqlPort := os.Getenv("MYSQL_PORT")
	if mysqlPort == "" {
		mysqlPort = "3306"
	}

	mysqlUser := os.Getenv("MYSQL_USER")
	if mysqlUser == "" {
		mysqlUser = "root"
	}

	mysqlPassword := os.Getenv("MYSQL_PASSWORD")
	mysqlDatabase := os.Getenv("MYSQL_DATABASE")
	if mysqlDatabase == "" {
		mysqlDatabase = "ecommerce"
	}

	mysqlMaxOpenConns := 25
	if raw := os.Getenv("MYSQL_MAX_OPEN_CONNS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			mysqlMaxOpenConns = parsed
		}
	}

	mysqlMaxIdleConns := 25
	if raw := os.Getenv("MYSQL_MAX_IDLE_CONNS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			mysqlMaxIdleConns = parsed
		}
	}

	mysqlConnMaxLifetimeMinutes := 5
	if raw := os.Getenv("MYSQL_CONN_MAX_LIFETIME_MINUTES"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			mysqlConnMaxLifetimeMinutes = parsed
		}
	}

	mysqlConnMaxIdleTimeMinutes := 5
	if raw := os.Getenv("MYSQL_CONN_MAX_IDLE_TIME_MINUTES"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			mysqlConnMaxIdleTimeMinutes = parsed
		}
	}

	rabbitMQHost := os.Getenv("RABBITMQ_HOST")
	if rabbitMQHost == "" {
		rabbitMQHost = "localhost"
	}

	rabbitMQPort := os.Getenv("RABBITMQ_PORT")
	if rabbitMQPort == "" {
		rabbitMQPort = "5672"
	}

	rabbitMQUser := os.Getenv("RABBITMQ_USER")
	if rabbitMQUser == "" {
		rabbitMQUser = "guest"
	}

	rabbitMQPassword := os.Getenv("RABBITMQ_PASSWORD")
	if rabbitMQPassword == "" {
		rabbitMQPassword = "guest"
	}

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	tokenRefreshPollMinutes := 5
	if raw := os.Getenv("TOKEN_REFRESH_POLL_INTERVAL_MINUTES"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			tokenRefreshPollMinutes = parsed
		}
	}

	tokenRefreshLookaheadMinutes := 15
	if raw := os.Getenv("TOKEN_REFRESH_LOOKAHEAD_MINUTES"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			tokenRefreshLookaheadMinutes = parsed
		}
	}

	syncQueuePollSeconds := 1800
	if raw := os.Getenv("SYNC_QUEUE_POLL_INTERVAL_SECONDS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			syncQueuePollSeconds = parsed
		}
	}

	syncQueueBatchSize := 10
	if raw := os.Getenv("SYNC_QUEUE_BATCH_SIZE"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			syncQueueBatchSize = parsed
		}
	}

	compatibilitiesFixRunAtHour := 12
	if raw := os.Getenv("COMPATIBILITIES_FIX_RUN_AT_HOUR"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 && parsed <= 23 {
			compatibilitiesFixRunAtHour = parsed
		}
	}

	compatibilitiesFixRunAtMinute := 0
	if raw := os.Getenv("COMPATIBILITIES_FIX_RUN_AT_MINUTE"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 && parsed <= 59 {
			compatibilitiesFixRunAtMinute = parsed
		}
	}

	listingDiscoveryRunAtHour := 1
	if raw := os.Getenv("LISTING_DISCOVERY_RUN_AT_HOUR"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 && parsed <= 23 {
			listingDiscoveryRunAtHour = parsed
		}
	}

	listingDiscoveryRunAtMinute := 0
	if raw := os.Getenv("LISTING_DISCOVERY_RUN_AT_MINUTE"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 && parsed <= 59 {
			listingDiscoveryRunAtMinute = parsed
		}
	}

	shutdownTimeoutSeconds := 25
	if raw := os.Getenv("SERVER_SHUTDOWN_TIMEOUT_SECONDS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			shutdownTimeoutSeconds = parsed
		}
	}

	sieAPIToken := os.Getenv("SIE_API_TOKEN")

	exchangeRateRunAtHour := 8
	if raw := os.Getenv("EXCHANGE_RATE_RUN_AT_HOUR"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 && parsed <= 23 {
			exchangeRateRunAtHour = parsed
		}
	}

	exchangeRateRunAtMinute := 0
	if raw := os.Getenv("EXCHANGE_RATE_RUN_AT_MINUTE"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 && parsed <= 59 {
			exchangeRateRunAtMinute = parsed
		}
	}

	return Config{
		Port:                port,
		ShutdownTimeout:     time.Duration(shutdownTimeoutSeconds) * time.Second,
		TLSKeystorePath:     tlsKeystorePath,
		TLSKeystorePassword: tlsKeystorePassword,
		TLSKeystoreType:     tlsKeystoreType,
		JWTSecret:           jwtSecret,
		JWTIssuer:           jwtIssuer,
		JWTTTLMinutes:       jwtTTLMinutes,
		MySQLHost:           mysqlHost,
		MySQLPort:           mysqlPort,
		MySQLUser:           mysqlUser,
		MySQLPassword:       mysqlPassword,
		MySQLDatabase:       mysqlDatabase,

		MySQLMaxOpenConns:    mysqlMaxOpenConns,
		MySQLMaxIdleConns:    mysqlMaxIdleConns,
		MySQLConnMaxLifetime: time.Duration(mysqlConnMaxLifetimeMinutes) * time.Minute,
		MySQLConnMaxIdleTime: time.Duration(mysqlConnMaxIdleTimeMinutes) * time.Minute,
		RabbitMQHost:         rabbitMQHost,
		RabbitMQPort:         rabbitMQPort,
		RabbitMQUser:         rabbitMQUser,
		RabbitMQPassword:     rabbitMQPassword,
		RedisHost:            redisHost,
		RedisPort:            redisPort,

		TokenRefreshPollInterval: time.Duration(tokenRefreshPollMinutes) * time.Minute,
		TokenRefreshLookahead:    time.Duration(tokenRefreshLookaheadMinutes) * time.Minute,

		SyncQueuePollInterval: time.Duration(syncQueuePollSeconds) * time.Second,
		SyncQueueBatchSize:    syncQueueBatchSize,

		ListingDiscoveryRunAtHour:   listingDiscoveryRunAtHour,
		ListingDiscoveryRunAtMinute: listingDiscoveryRunAtMinute,

		CompatibilitiesFixRunAtHour:   compatibilitiesFixRunAtHour,
		CompatibilitiesFixRunAtMinute: compatibilitiesFixRunAtMinute,

		SIEAPIToken:             sieAPIToken,
		ExchangeRateRunAtHour:   exchangeRateRunAtHour,
		ExchangeRateRunAtMinute: exchangeRateRunAtMinute,
	}
}
