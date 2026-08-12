package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port string

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

	// SyncQueuePollInterval and SyncQueueBatchSize configure the
	// marketplace sync worker. See internal/workers/marketplace_worker.go.
	SyncQueuePollInterval time.Duration
	SyncQueueBatchSize    int

	// CompatibilitiesFixRunAtHour/CompatibilitiesFixRunAtMinute configure the
	// server-local time of day CompatibilitiesFixScheduler runs once a day,
	// re-checking every active MERCADOLIBRE connection for listings still
	// needing vehicle compatibilities pushed (see
	// internal/interfaces/schedulers/compatibilities_fix_scheduler.go). Each
	// run can cost up to ~20 MercadoLibre API calls per connection
	// (FixUnderReviewListings' default limit of 10 rows, ~2 calls/row),
	// against a 10-calls/minute budget shared by the whole app.
	CompatibilitiesFixRunAtHour   int
	CompatibilitiesFixRunAtMinute int
}

func Load() Config {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8081"
	}

	tlsKeystorePath := os.Getenv("SERVER_TLS_KEYSTORE_PATH")
	tlsKeystorePassword := os.Getenv("SERVER_TLS_KEYSTORE_PASSWORD")
	tlsKeystoreType := os.Getenv("SERVER_TLS_KEYSTORE_TYPE")
	if tlsKeystoreType == "" {
		tlsKeystoreType = "PKCS12"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "change-me-in-production"
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

	syncQueuePollSeconds := 30
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

	return Config{
		Port:                port,
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
		RabbitMQHost:        rabbitMQHost,
		RabbitMQPort:        rabbitMQPort,
		RabbitMQUser:        rabbitMQUser,
		RabbitMQPassword:    rabbitMQPassword,
		RedisHost:           redisHost,
		RedisPort:           redisPort,

		TokenRefreshPollInterval: time.Duration(tokenRefreshPollMinutes) * time.Minute,
		TokenRefreshLookahead:    time.Duration(tokenRefreshLookaheadMinutes) * time.Minute,

		SyncQueuePollInterval: time.Duration(syncQueuePollSeconds) * time.Second,
		SyncQueueBatchSize:    syncQueueBatchSize,

		CompatibilitiesFixRunAtHour:   compatibilitiesFixRunAtHour,
		CompatibilitiesFixRunAtMinute: compatibilitiesFixRunAtMinute,
	}
}
