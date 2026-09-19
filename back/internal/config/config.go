package config

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath = "config/config.yaml"
	configPathEnv     = "CONFIG_PATH"

	databaseURLOverrideEnv = "DATABASE_URL"
	redisURLOverrideEnv    = "REDIS_URL"
)

// Config is the process configuration: structure and tuning from YAML, secrets
// from the environment variables the YAML names.
type Config struct {
	Env      string         `yaml:"env" validate:"required"`
	HTTP     HTTPConfig     `yaml:"http" validate:"required"`
	Database DatabaseConfig `yaml:"database" validate:"required"`
	Redis    RedisConfig    `yaml:"redis" validate:"required"`
	Auth     AuthConfig     `yaml:"auth" validate:"required"`

	databaseURL string
	redisURL    string
	authSecret  string
}

type HTTPConfig struct {
	Port              int           `yaml:"port" validate:"required,min=1,max=65535"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout" validate:"required,gt=0"`
	ReadTimeout       time.Duration `yaml:"read_timeout" validate:"required,gt=0"`
	WriteTimeout      time.Duration `yaml:"write_timeout" validate:"required,gt=0"`
	IdleTimeout       time.Duration `yaml:"idle_timeout" validate:"required,gt=0"`
	RequestTimeout    time.Duration `yaml:"request_timeout" validate:"required,gt=0"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout" validate:"required,gt=0"`
}

type DatabaseConfig struct {
	Host              string        `yaml:"host" validate:"required,hostname|ip"`
	Port              int           `yaml:"port" validate:"required,min=1,max=65535"`
	Name              string        `yaml:"name" validate:"required"`
	User              string        `yaml:"user" validate:"required"`
	PasswordKey       string        `yaml:"password_key" validate:"required"`
	SSLMode           string        `yaml:"ssl_mode" validate:"required,oneof=disable allow prefer require verify-ca verify-full"`
	MaxConns          int32         `yaml:"max_conns" validate:"required,gt=0"`
	MinConns          int32         `yaml:"min_conns" validate:"gte=0,ltefield=MaxConns"`
	MaxConnLifetime   time.Duration `yaml:"max_conn_lifetime" validate:"required,gt=0"`
	MaxConnIdleTime   time.Duration `yaml:"max_conn_idle_time" validate:"required,gt=0"`
	HealthCheckPeriod time.Duration `yaml:"health_check_period" validate:"required,gt=0"`
	MigrateOnBoot     bool          `yaml:"migrate_on_boot"`
}

type RedisConfig struct {
	Host        string        `yaml:"host" validate:"required,hostname|ip"`
	Port        int           `yaml:"port" validate:"required,min=1,max=65535"`
	DB          int           `yaml:"db" validate:"gte=0"`
	PasswordKey string        `yaml:"password_key" validate:"required"`
	DefaultTTL  time.Duration `yaml:"default_ttl" validate:"required,gt=0"`
}

type AuthConfig struct {
	SecretKey  string          `yaml:"secret_key" validate:"required"`
	AccessTTL  time.Duration   `yaml:"access_ttl" validate:"required,gt=0"`
	RefreshTTL time.Duration   `yaml:"refresh_ttl" validate:"required,gt=0,gtfield=AccessTTL"`
	RateLimit  RateLimitConfig `yaml:"rate_limit" validate:"required"`
}

// RateLimitConfig bounds what one caller may ask for. The two credential
// ceilings are separate because one address trying many accounts and many
// addresses trying one account are different attacks and neither counter sees
// the other; the read ceiling is a different concern again — not guessing, but
// one signed-in session repeating an expensive query.
type RateLimitConfig struct {
	AddressAttempts int           `yaml:"address_attempts" validate:"required,gt=0"`
	AddressWindow   time.Duration `yaml:"address_window" validate:"required,gt=0"`
	AccountAttempts int           `yaml:"account_attempts" validate:"required,gt=0"`
	AccountWindow   time.Duration `yaml:"account_window" validate:"required,gt=0"`
	ReadAttempts    int           `yaml:"read_attempts" validate:"required,gt=0"`
	ReadWindow      time.Duration `yaml:"read_window" validate:"required,gt=0"`
}

// ConfigPath resolves the configuration file to read: the flag value when given,
// then CONFIG_PATH, then the default beside the binary.
func ConfigPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if fromEnv := os.Getenv(configPathEnv); fromEnv != "" {
		return fromEnv
	}
	return defaultConfigPath
}

// Load reads the configuration file and resolves every secret it names.
func Load(path string) (Config, error) { return load(path, requireAllSecrets) }

// LoadForMigrations reads the configuration but requires only the database
// secret, so the migration command runs without Redis or a signing key.
func LoadForMigrations(path string) (Config, error) { return load(path, requireDatabaseSecret) }

// LoadStructure reads the configuration without resolving any secret, for
// tooling that only needs the shape — the health probe reads the port with it.
func LoadStructure(path string) (Config, error) { return load(path, requireNoSecrets) }

type secretRequirement int

const (
	requireNoSecrets secretRequirement = iota
	requireDatabaseSecret
	requireAllSecrets
)

func load(path string, required secretRequirement) (Config, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- the path comes from an operator flag or CONFIG_PATH, never from a request
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	cfg.applyDefaults()

	if err := validateStructure(&cfg); err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	if err := cfg.resolveSecrets(required); err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Env == "" {
		c.Env = "development"
	}
	if c.HTTP.Port == 0 {
		c.HTTP.Port = 8080
	}
	if c.Database.MaxConns <= 0 {
		c.Database.MaxConns = int32(runtime.NumCPU() * 4)
	}
	if c.Database.SSLMode == "" {
		c.Database.SSLMode = "disable"
	}
}

func (c *Config) resolveSecrets(required secretRequirement) error {
	var missing []string

	password, err := secret(c.Database.PasswordKey)
	if err != nil && required >= requireDatabaseSecret {
		missing = append(missing, fmt.Sprintf("database.password_key -> %s", c.Database.PasswordKey))
	}

	var redisPassword, authSecret string
	if required == requireAllSecrets {
		redisPassword, err = secret(c.Redis.PasswordKey)
		if err != nil {
			missing = append(missing, fmt.Sprintf("redis.password_key -> %s", c.Redis.PasswordKey))
		}
		authSecret, err = secret(c.Auth.SecretKey)
		if err != nil {
			missing = append(missing, fmt.Sprintf("auth.secret_key -> %s", c.Auth.SecretKey))
		}
	}

	c.databaseURL = os.Getenv(databaseURLOverrideEnv)
	c.redisURL = os.Getenv(redisURLOverrideEnv)
	c.authSecret = authSecret

	if c.databaseURL != "" {
		missing = discard(missing, "database.password_key")
	} else {
		c.databaseURL = buildPostgresURL(c.Database, password)
	}
	if c.redisURL != "" {
		missing = discard(missing, "redis.password_key")
	} else {
		c.redisURL = buildRedisURL(c.Redis, redisPassword)
	}

	if len(missing) > 0 {
		return fmt.Errorf("environment variables named by the config are empty: %v", missing)
	}
	return nil
}

func secret(key string) (string, error) {
	if key == "" {
		return "", errors.New("no environment variable named")
	}
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s is empty", key)
	}
	return value, nil
}

func discard(values []string, prefix string) []string {
	kept := values[:0]
	for _, v := range values {
		if len(v) < len(prefix) || v[:len(prefix)] != prefix {
			kept = append(kept, v)
		}
	}
	return kept
}

func buildPostgresURL(cfg DatabaseConfig, password string) string {
	host := cfg.Host
	if cfg.Port != 0 {
		host = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	}
	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, password),
		Host:     host,
		Path:     "/" + cfg.Name,
		RawQuery: "sslmode=" + url.QueryEscape(cfg.SSLMode),
	}
	return dsn.String()
}

func buildRedisURL(cfg RedisConfig, password string) string {
	host := cfg.Host
	if cfg.Port != 0 {
		host = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	}
	redisURL := url.URL{
		Scheme: "redis",
		User:   url.UserPassword("", password),
		Host:   host,
		Path:   "/" + strconv.Itoa(cfg.DB),
	}
	return redisURL.String()
}

func (c *Config) DatabaseURL() string { return c.databaseURL }

func (c *Config) RedisURL() string { return c.redisURL }

func (c *Config) AuthSecret() string { return c.authSecret }

// Development reports whether the process runs outside production.
func (c *Config) Development() bool { return c.Env != "production" }

func (c *Config) Addr() string { return ":" + strconv.Itoa(c.HTTP.Port) }

// LogValue renders the configuration without its secrets.
func (c *Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("env", c.Env),
		slog.Int("http.port", c.HTTP.Port),
		slog.String("database.host", c.Database.Host),
		slog.String("database.name", c.Database.Name),
		slog.Int("database.maxConns", int(c.Database.MaxConns)),
		slog.String("redis.host", c.Redis.Host),
		slog.Bool("database.migrateOnBoot", c.Database.MigrateOnBoot),
	)
}
