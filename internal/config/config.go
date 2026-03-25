package config

import (
	"fmt"
	"time"

	cleanenvport "github.com/wb-go/wbf/config/cleanenv-port"
)

const defaultConfigPath = "configs/config.yaml"

type HTTPConfig struct {
	Addr                   string `yaml:"addr" env:"HTTP_ADDR" env-default:":8080" validate:"required"`
	ReadTimeoutSeconds     int    `yaml:"read_timeout_seconds" env:"HTTP_READ_TIMEOUT_SECONDS" env-default:"10" validate:"gte=1"`
	WriteTimeoutSeconds    int    `yaml:"write_timeout_seconds" env:"HTTP_WRITE_TIMEOUT_SECONDS" env-default:"10" validate:"gte=1"`
	IdleTimeoutSeconds     int    `yaml:"idle_timeout_seconds" env:"HTTP_IDLE_TIMEOUT_SECONDS" env-default:"30" validate:"gte=1"`
	ShutdownTimeoutSeconds int    `yaml:"shutdown_timeout_seconds" env:"HTTP_SHUTDOWN_TIMEOUT_SECONDS" env-default:"10" validate:"gte=1"`
}

type PostgresConfig struct {
	DSN          string `yaml:"dsn" env:"POSTGRES_DSN" validate:"required"`
	MaxPoolSize  int32  `yaml:"max_pool_size" env:"POSTGRES_MAX_POOL_SIZE" env-default:"10" validate:"gt=0"`
	ConnAttempts int    `yaml:"conn_attempts" env:"POSTGRES_CONN_ATTEMPTS" env-default:"5" validate:"gt=0"`
}

type LoggerConfig struct {
	Engine string `yaml:"engine" env:"LOGGER_ENGINE" env-default:"slog" validate:"required"`
	Level  string `yaml:"level" env:"LOGGER_LEVEL" env-default:"info" validate:"required"`
}

type TransactionConfig struct {
	MaxAttempts      int `yaml:"max_attempts" env:"TX_MAX_ATTEMPTS" env-default:"3" validate:"gt=0"`
	BaseRetryDelayMs int `yaml:"base_retry_delay_ms" env:"TX_BASE_RETRY_DELAY_MS" env-default:"10" validate:"gt=0"`
	MaxRetryDelayMs  int `yaml:"max_retry_delay_ms" env:"TX_MAX_RETRY_DELAY_MS" env-default:"100" validate:"gt=0"`
}

type AuthConfig struct {
	JWTSecret           string `yaml:"jwt_secret" env:"AUTH_JWT_SECRET" validate:"required"`
	AccessTTLMinutes    int    `yaml:"access_ttl_minutes" env:"AUTH_ACCESS_TTL_MINUTES" env-default:"15" validate:"gt=0"`
	RefreshTTLHours     int    `yaml:"refresh_ttl_hours" env:"AUTH_REFRESH_TTL_HOURS" env-default:"168" validate:"gt=0"`
	Issuer              string `yaml:"issuer" env:"AUTH_ISSUER" env-default:"eventbooker" validate:"required"`
	RefreshCookieName   string `yaml:"refresh_cookie_name" env:"AUTH_REFRESH_COOKIE_NAME" env-default:"refresh_token" validate:"required"`
	RefreshCookieSecure bool   `yaml:"refresh_cookie_secure" env:"AUTH_REFRESH_COOKIE_SECURE" env-default:"false"`
}

type AppConfig struct {
	Name        string            `yaml:"name" env:"APP_NAME" env-default:"eventbooker" validate:"required"`
	Env         string            `yaml:"env" env:"APP_ENV" env-default:"dev" validate:"required"`
	HTTP        HTTPConfig        `yaml:"http" validate:"required"`
	Postgres    PostgresConfig    `yaml:"postgres" validate:"required"`
	Logger      LoggerConfig      `yaml:"logger" validate:"required"`
	Auth        AuthConfig        `yaml:"auth" validate:"required"`
	Transaction TransactionConfig `yaml:"transaction" validate:"required"`
}

type rootConfig struct {
	App AppConfig `yaml:"app" validate:"required"`
}

func LoadAppConfig() (*AppConfig, error) {
	var root rootConfig
	if err := cleanenvport.LoadPath(defaultConfigPath, &root); err != nil {
		return nil, fmt.Errorf("load app config: %w", err)
	}
	return &root.App, nil
}

func (c HTTPConfig) ReadTimeout() time.Duration {
	return time.Duration(c.ReadTimeoutSeconds) * time.Second
}

func (c HTTPConfig) WriteTimeout() time.Duration {
	return time.Duration(c.WriteTimeoutSeconds) * time.Second
}

func (c HTTPConfig) IdleTimeout() time.Duration {
	return time.Duration(c.IdleTimeoutSeconds) * time.Second
}

func (c HTTPConfig) ShutdownTimeout() time.Duration {
	return time.Duration(c.ShutdownTimeoutSeconds) * time.Second
}

func (c TransactionConfig) BaseRetryDelay() time.Duration {
	return time.Duration(c.BaseRetryDelayMs) * time.Millisecond
}

func (c TransactionConfig) MaxRetryDelay() time.Duration {
	return time.Duration(c.MaxRetryDelayMs) * time.Millisecond
}

func (c AuthConfig) AccessTTL() time.Duration {
	return time.Duration(c.AccessTTLMinutes) * time.Minute
}

func (c AuthConfig) RefreshTTL() time.Duration {
	return time.Duration(c.RefreshTTLHours) * time.Hour
}
