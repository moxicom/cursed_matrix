package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moxicom/cursed_matrix/back/internal/config"
)

const sampleConfig = `
env: production
http:
  port: 9090
  read_header_timeout: 5s
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 2m
  request_timeout: 1m
  shutdown_timeout: 15s
database:
  host: postgres
  port: 5432
  name: cursed_matrix
  user: cursed
  password_key: TEST_PG_PASSWORD
  ssl_mode: require
  max_conns: 8
  min_conns: 2
  max_conn_lifetime: 1h
  max_conn_idle_time: 15m
  health_check_period: 30s
  migrate_on_boot: true
redis:
  host: redis
  port: 6379
  db: 3
  password_key: TEST_REDIS_PASSWORD
  default_ttl: 10m
auth:
  secret_key: TEST_AUTH_SECRET
  access_ttl: 15m
  refresh_ttl: 720h
  rate_limit:
    address_attempts: 20
    address_window: 5m
    account_attempts: 5
    account_window: 15m
    register_attempts: 5
    register_window: 1h
    read_attempts: 300
    read_window: 1m
    write_attempts: 240
    write_window: 1m

billing:
  enabled: false
  trial_period: 336h
  granted_period: 720h
  prices:
    - language: EN
      currency: USD
      amount: 500
`

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func setSecrets(t *testing.T) {
	t.Helper()
	t.Setenv("TEST_PG_PASSWORD", "pg-secret")
	t.Setenv("TEST_REDIS_PASSWORD", "redis-secret")
	t.Setenv("TEST_AUTH_SECRET", "auth-secret")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "")
	// Like the two above: the environment overrides the file, and the shell
	// running the tests may well carry the project's own .env.
	t.Setenv("APP_ENV", "")
}

func TestLoadReadsStructureAndResolvesSecrets(t *testing.T) {
	setSecrets(t)
	cfg, err := config.Load(writeConfig(t, sampleConfig))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{name: "env", got: cfg.Env, want: "production"},
		{name: "development flag", got: cfg.Development(), want: false},
		{name: "listen address", got: cfg.Addr(), want: ":9090"},
		{name: "idle timeout", got: cfg.HTTP.IdleTimeout, want: 2 * time.Minute},
		{name: "shutdown timeout", got: cfg.HTTP.ShutdownTimeout, want: 15 * time.Second},
		{name: "pool ceiling", got: cfg.Database.MaxConns, want: int32(8)},
		{name: "migrate on boot", got: cfg.Database.MigrateOnBoot, want: true},
		{name: "cache ttl", got: cfg.Redis.DefaultTTL, want: 10 * time.Minute},
		{name: "access token ttl", got: cfg.Auth.AccessTTL, want: 15 * time.Minute},
		{name: "attempts per address", got: cfg.Auth.RateLimit.AddressAttempts, want: 20},
		{name: "attempts per account", got: cfg.Auth.RateLimit.AccountAttempts, want: 5},
		{name: "account window", got: cfg.Auth.RateLimit.AccountWindow, want: 15 * time.Minute},
		{
			name: "database url", got: cfg.DatabaseURL(),
			want: "postgres://cursed:pg-secret@postgres:5432/cursed_matrix?sslmode=require",
		},
		{name: "redis url", got: cfg.RedisURL(), want: "redis://:redis-secret@redis:6379/3"},
		{name: "auth secret", got: cfg.AuthSecret(), want: "auth-secret"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestLoadRejectsBrokenConfigurations(t *testing.T) {
	tests := []struct {
		name string
		body string
		// env lists variables to blank out before loading.
		env      []string
		wantWord string
	}{
		{
			name:     "unknown field",
			body:     sampleConfig + "\nunexpected: true\n",
			wantWord: "unexpected",
		},
		{
			name: "duration that is not a duration",
			body: strings.Replace(sampleConfig, "read_timeout: 30s", "read_timeout: soon", 1),
		},
		{
			name: "port that is not a number",
			body: strings.Replace(sampleConfig, "port: 9090", "port: eighty", 1),
		},
		{
			name:     "database host missing",
			body:     strings.Replace(sampleConfig, "  host: postgres\n", "", 1),
			wantWord: "database.host is required",
		},
		{
			name:     "no attempt ceiling",
			body:     strings.Replace(sampleConfig, "account_attempts: 5", "account_attempts: 0", 1),
			wantWord: "auth.rate_limit.account_attempts",
		},
		{
			name:     "a price with no currency",
			body:     strings.Replace(sampleConfig, "currency: USD", "currency: ''", 1),
			wantWord: "billing.prices[0].currency",
		},
		{
			name:     "a granted period of zero",
			body:     strings.Replace(sampleConfig, "granted_period: 720h", "granted_period: 0s", 1),
			wantWord: "billing.granted_period",
		},
		{
			name:     "read ceiling zero",
			body:     strings.Replace(sampleConfig, "read_attempts: 300", "read_attempts: 0", 1),
			wantWord: "auth.rate_limit.read_attempts",
		},
		{
			name:     "write ceiling zero",
			body:     strings.Replace(sampleConfig, "write_attempts: 240", "write_attempts: 0", 1),
			wantWord: "auth.rate_limit.write_attempts",
		},
		{
			// Without a ceiling here one address can manufacture accounts, and
			// a config that forgot it must not start.
			name:     "registration ceiling zero",
			body:     strings.Replace(sampleConfig, "register_attempts: 5", "register_attempts: 0", 1),
			wantWord: "auth.rate_limit.register_attempts",
		},
		{
			name:     "unknown ssl mode",
			body:     strings.Replace(sampleConfig, "ssl_mode: require", "ssl_mode: maybe", 1),
			wantWord: "database.ssl_mode must be one of",
		},
		{
			name:     "port out of range",
			body:     strings.Replace(sampleConfig, "port: 9090", "port: 70000", 1),
			wantWord: "http.port must be at most 65535",
		},
		{
			name:     "minimum pool above the maximum",
			body:     strings.Replace(sampleConfig, "min_conns: 2", "min_conns: 99", 1),
			wantWord: "database.min_conns must not exceed max_conns",
		},
		{
			name:     "refresh token shorter than the access token",
			body:     strings.Replace(sampleConfig, "refresh_ttl: 720h", "refresh_ttl: 1m", 1),
			wantWord: "auth.refresh_ttl must be greater than access_ttl",
		},
		{
			name:     "zero timeout",
			body:     strings.Replace(sampleConfig, "read_timeout: 30s", "read_timeout: 0s", 1),
			wantWord: "http.read_timeout is required",
		},
		{
			name:     "host that is not a host",
			body:     strings.Replace(sampleConfig, "host: postgres", "host: \"not a host\"", 1),
			wantWord: "database.host",
		},
		{
			name:     "auth secret variable empty",
			body:     sampleConfig,
			env:      []string{"TEST_AUTH_SECRET"},
			wantWord: "auth.secret_key",
		},
		{
			name:     "database password variable empty",
			body:     sampleConfig,
			env:      []string{"TEST_PG_PASSWORD"},
			wantWord: "TEST_PG_PASSWORD",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setSecrets(t)
			for _, key := range tc.env {
				t.Setenv(key, "")
			}

			_, err := config.Load(writeConfig(t, tc.body))
			if err == nil {
				t.Fatal("Load accepted a configuration it must reject")
			}
			if tc.wantWord != "" && !strings.Contains(err.Error(), tc.wantWord) {
				t.Errorf("the error does not mention %q: %v", tc.wantWord, err)
			}
		})
	}
}

func TestSecretRequirementsPerEntryPoint(t *testing.T) {
	tests := []struct {
		name    string
		load    func(path string) (config.Config, error)
		blank   []string
		wantErr bool
	}{
		{
			name: "the service needs every secret",
			load: config.Load, blank: []string{"TEST_AUTH_SECRET"}, wantErr: true,
		},
		{
			name: "migrations need only the database",
			load: config.LoadForMigrations, blank: []string{"TEST_AUTH_SECRET", "TEST_REDIS_PASSWORD"},
		},
		{
			name: "migrations still need the database password",
			load: config.LoadForMigrations, blank: []string{"TEST_PG_PASSWORD"}, wantErr: true,
		},
		{
			name: "the health probe needs none",
			load: config.LoadStructure,
			blank: []string{
				"TEST_PG_PASSWORD", "TEST_REDIS_PASSWORD", "TEST_AUTH_SECRET",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setSecrets(t)
			for _, key := range tc.blank {
				t.Setenv(key, "")
			}

			cfg, err := tc.load(writeConfig(t, sampleConfig))
			if (err != nil) != tc.wantErr {
				t.Fatalf("load error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && cfg.HTTP.Port != 9090 {
				t.Errorf("port = %d, want the value from the file", cfg.HTTP.Port)
			}
		})
	}
}

func TestPasswordsAreEncodedIntoTheURL(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantIn   string
	}{
		{name: "plain", password: "simple", wantIn: "cursed:simple@"},
		{name: "at sign and slash", password: "p@ss/word", wantIn: "p%40ss%2Fword"},
		{name: "colon", password: "pass:word", wantIn: "pass%3Aword"},
		{name: "percent", password: "100%pass", wantIn: "100%25pass"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setSecrets(t)
			t.Setenv("TEST_PG_PASSWORD", tc.password)

			cfg, err := config.Load(writeConfig(t, sampleConfig))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !strings.Contains(cfg.DatabaseURL(), tc.wantIn) {
				t.Errorf("DatabaseURL() = %q, want it to contain %q", cfg.DatabaseURL(), tc.wantIn)
			}
		})
	}
}

func TestURLOverridesWin(t *testing.T) {
	tests := []struct {
		name     string
		variable string
		value    string
		read     func(cfg *config.Config) string
	}{
		{
			name: "database", variable: "DATABASE_URL",
			value: "postgres://other:pass@127.0.0.1:5433/other?sslmode=disable",
			read:  (*config.Config).DatabaseURL,
		},
		{
			name: "redis", variable: "REDIS_URL",
			value: "redis://:other@127.0.0.1:6380/7",
			read:  (*config.Config).RedisURL,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setSecrets(t)
			t.Setenv(tc.variable, tc.value)

			cfg, err := config.Load(writeConfig(t, sampleConfig))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := tc.read(&cfg); got != tc.value {
				t.Errorf("%s = %q, want the override %q", tc.name, got, tc.value)
			}
		})
	}
}

func TestLogValueHidesSecrets(t *testing.T) {
	setSecrets(t)
	cfg, err := config.Load(writeConfig(t, sampleConfig))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rendered := cfg.LogValue().String()

	tests := []struct {
		name    string
		needle  string
		present bool
	}{
		{name: "database password", needle: "pg-secret"},
		{name: "redis password", needle: "redis-secret"},
		{name: "auth secret", needle: "auth-secret"},
		{name: "database name is kept", needle: "cursed_matrix", present: true},
		{name: "host is kept", needle: "postgres", present: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Contains(rendered, tc.needle) != tc.present {
				t.Errorf("LogValue() = %s; %q present = %v, want %v",
					rendered, tc.needle, !tc.present, tc.present)
			}
		})
	}
}

func TestConfigPathPrecedence(t *testing.T) {
	tests := []struct {
		name string
		flag string
		env  string
		want string
	}{
		{name: "flag wins", flag: "from/flag.yaml", env: "from/env.yaml", want: "from/flag.yaml"},
		{name: "env when no flag", flag: "", env: "from/env.yaml", want: "from/env.yaml"},
		{name: "default when neither", flag: "", env: "", want: "config/config.yaml"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CONFIG_PATH", tc.env)
			if got := config.ConfigPath(tc.flag); got != tc.want {
				t.Errorf("ConfigPath(%q) = %q, want %q", tc.flag, got, tc.want)
			}
		})
	}
}

func TestMissingFileIsReported(t *testing.T) {
	setSecrets(t)
	if _, err := config.Load(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Fatal("Load accepted a path that does not exist")
	}
}

func TestShippedConfigLoads(t *testing.T) {
	setSecrets(t)
	t.Setenv("POSTGRES_PASSWORD", "pg")
	t.Setenv("REDIS_PASSWORD", "redis")
	t.Setenv("JWT_SECRET", "jwt")

	if _, err := config.Load("../../config/config.yaml"); err != nil {
		t.Fatalf("the configuration shipped in the repository does not load: %v", err)
	}
}

// The YAML is committed and the same file is mounted on every host, so the
// deployment has to be able to say it is production without editing it.
// Getting this wrong is quiet and expensive: Development() decides whether the
// session cookie carries Secure.
func TestEnvironmentComesFromTheEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		appEnv      string
		wantEnv     string
		wantDevMode bool
	}{
		{name: "unset leaves the file's value", appEnv: "", wantEnv: "production", wantDevMode: false},
		{name: "production turns off development", appEnv: "production", wantEnv: "production", wantDevMode: false},
		{name: "anything but production is development", appEnv: "staging", wantEnv: "staging", wantDevMode: true},
		{name: "surrounding space is trimmed", appEnv: "  production  ", wantEnv: "production", wantDevMode: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setSecrets(t)
			// Set in every case: an inherited value from the shell would make
			// the "unset" case pass or fail for the wrong reason.
			t.Setenv("APP_ENV", tt.appEnv)

			cfg, err := config.Load(writeConfig(t, sampleConfig))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			if cfg.Env != tt.wantEnv {
				t.Errorf("Env = %q, want %q", cfg.Env, tt.wantEnv)
			}
			if cfg.Development() != tt.wantDevMode {
				t.Errorf("Development() = %v, want %v — this decides the Secure cookie flag",
					cfg.Development(), tt.wantDevMode)
			}
		})
	}
}
