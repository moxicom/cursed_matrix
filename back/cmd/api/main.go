// Command api serves the HTTP API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	httphandler "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler"
	httpserver "github.com/moxicom/cursed_matrix/back/internal/adapter/http-server"
	"github.com/moxicom/cursed_matrix/back/internal/adapter/metrics"
	"github.com/moxicom/cursed_matrix/back/internal/adapter/postgres"
	redisadapter "github.com/moxicom/cursed_matrix/back/internal/adapter/redis"
	"github.com/moxicom/cursed_matrix/back/internal/adapter/token"
	"github.com/moxicom/cursed_matrix/back/internal/app/auth"
	"github.com/moxicom/cursed_matrix/back/internal/app/board"
	"github.com/moxicom/cursed_matrix/back/internal/config"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/pkg/utils"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "path to the YAML configuration file")
	probe := flag.Bool("healthcheck", false, "probe the running server and exit")
	flag.Parse()

	if *probe {
		return healthcheck(config.ConfigPath(*configPath))
	}

	cfg, err := config.Load(config.ConfigPath(*configPath))
	if err != nil {
		return err
	}
	log := utils.NewLogger(utils.LoggerOptions{Env: cfg.Env, Verbose: cfg.Development()})
	// The process default is a fallback for code that never sees a request
	// context; components below are handed their own tagged logger.
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, postgres.Options{
		URL:               cfg.DatabaseURL(),
		MaxConns:          cfg.Database.MaxConns,
		MinConns:          cfg.Database.MinConns,
		MaxConnLifetime:   cfg.Database.MaxConnLifetime,
		MaxConnIdleTime:   cfg.Database.MaxConnIdleTime,
		HealthCheckPeriod: cfg.Database.HealthCheckPeriod,
	}, utils.ForComponent(log, "postgres"))
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}

	redisCache, err := redisadapter.NewCache(ctx, redisadapter.Options{
		URL:        cfg.RedisURL(),
		DefaultTTL: cfg.Redis.DefaultTTL,
	}, utils.ForComponent(log, "redis"))
	if err != nil {
		log.Warn("cache disabled", "err", err)
		redisCache = nil
	}

	registry := metrics.New()
	registry.MustRegister(metrics.PoolCollector(pool))

	var api http.Handler
	if redisCache != nil {
		tokens := token.NewJWTIssuer(cfg.AuthSecret(), cfg.Auth.AccessTTL, &shared.SystemClock{})
		service := auth.NewService(
			postgres.NewUserRepository(pool),
			redisadapter.NewRefreshStore(redisCache.Client()),
			postgres.NewTxManager(pool, utils.ForComponent(log, "postgres")),
			tokens,
			redisCache,
			&shared.SystemClock{},
			cfg.Auth.RefreshTTL,
		)
		boards := board.NewService(
			postgres.NewTaskRepository(pool),
			postgres.NewUserRepository(pool),
			redisCache,
			&shared.SystemClock{},
		)
		limiter := redisadapter.NewRateLimiter(redisCache.Client(), utils.ForComponent(log, "ratelimit"))
		limits := httphandler.RateLimits{
			AddressAttempts: cfg.Auth.RateLimit.AddressAttempts,
			AddressWindow:   cfg.Auth.RateLimit.AddressWindow,
			AccountAttempts: cfg.Auth.RateLimit.AccountAttempts,
			AccountWindow:   cfg.Auth.RateLimit.AccountWindow,
			ReadAttempts:    cfg.Auth.RateLimit.ReadAttempts,
			ReadWindow:      cfg.Auth.RateLimit.ReadWindow,
		}
		api = httphandler.Routes(
			httphandler.NewAPI(service, boards, httphandler.NewCookieWriter(!cfg.Development()),
				cfg.Auth.RefreshTTL, limiter, limits),
			tokens, limiter, limits,
		)
	} else {
		log.Warn("api disabled: the refresh store needs Redis")
	}

	router := httpserver.NewRouter(&cfg, utils.ForComponent(log, "http"), registry.Registry, func(ctx context.Context) error {
		return pool.Ping(ctx)
	}, api, registry.Middleware)

	server := httpserver.NewServer(&cfg, router, utils.ForComponent(log, "http"))
	serving := make(chan error, 1)
	go func() { serving <- server.Serve() }()

	log.Info("starting", "version", version, "config", cfg)

	var serveErr error
	select {
	case serveErr = <-serving:
		log.Error("http server stopped on its own", "err", serveErr)
	case <-ctx.Done():
		// Restoring the default handler makes a second signal kill the process
		// instead of being swallowed while the first shutdown is still running.
		stop()
		log.Info("shutdown signal received", "budget", cfg.HTTP.ShutdownTimeout)
	}

	shutdownErr := shutdown(cfg.HTTP.ShutdownTimeout, log, []step{
		{name: "http server", stop: server.Shutdown},
		{name: "cache", stop: closing(func() error {
			if redisCache == nil {
				return nil
			}
			return redisCache.Close()
		})},
		{name: "database", stop: closing(func() error { pool.Close(); return nil })},
	})

	return errors.Join(serveErr, shutdownErr)
}

// step is one thing to stop, in the order the process stops it: the listener
// first, so nothing new arrives, then what the in-flight requests were using.
type step struct {
	name string
	stop func(context.Context) error
}

// shutdown spends one budget across every step and never blocks past it: a
// dependency that refuses to close must not keep the process alive.
func shutdown(budget time.Duration, log *slog.Logger, steps []step) error {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	var failures []error
	for _, s := range steps {
		started := time.Now()
		if err := s.stop(ctx); err != nil {
			log.Error("shutdown step failed", "step", s.name, "err", err)
			failures = append(failures, fmt.Errorf("%s: %w", s.name, err))
			continue
		}
		log.Info("stopped", "step", s.name, "tookMs", time.Since(started).Milliseconds())
	}

	if len(failures) == 0 {
		log.Info("shutdown complete")
	}
	return errors.Join(failures...)
}

// closing adapts a Close that takes no context to the deadline the process set.
func closing(close func() error) func(context.Context) error {
	return func(ctx context.Context) error {
		done := make(chan error, 1)
		go func() { done <- close() }()

		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func healthcheck(configPath string) error {
	cfg, err := config.LoadStructure(configPath)
	if err != nil {
		return err
	}

	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", cfg.HTTP.Port))
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz answered %d", response.StatusCode)
	}
	return nil
}
