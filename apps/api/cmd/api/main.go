// Command api is the entrypoint for the FinWatch modular monolith HTTP service.
//
// FinWatch is an open-source reference implementation for near-real-time
// transaction monitoring and operational alert workflows using synthetic data.
// This binary boots a single process that will, in later issues, host all
// feature modules behind one HTTP surface.
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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vianbas/finwatch/apps/api/internal/alerts"
	alerthttp "github.com/vianbas/finwatch/apps/api/internal/alerts/httpapi"
	alertstore "github.com/vianbas/finwatch/apps/api/internal/alerts/store"
	"github.com/vianbas/finwatch/apps/api/internal/config"
	"github.com/vianbas/finwatch/apps/api/internal/platform/httpserver"
	"github.com/vianbas/finwatch/apps/api/internal/platform/postgres"
	"github.com/vianbas/finwatch/apps/api/internal/rules"
	rulestore "github.com/vianbas/finwatch/apps/api/internal/rules/store"
	"github.com/vianbas/finwatch/apps/api/internal/transactions"
	txhttp "github.com/vianbas/finwatch/apps/api/internal/transactions/httpapi"
	txstore "github.com/vianbas/finwatch/apps/api/internal/transactions/store"
)

func main() {
	// `api healthcheck` is a self-probe used by the container HEALTHCHECK, so the
	// distroless runtime image needs no shell or curl. It exits 0 when healthy.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			fmt.Fprintln(os.Stderr, "healthcheck failed:", err)
			os.Exit(1)
		}
		return
	}

	// `api seed -n N` ingests N synthetic transactions and exits.
	if len(os.Args) > 1 && os.Args[1] == "seed" {
		if err := runSeed(os.Args[2:]); err != nil {
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		// run already logged the cause; this is the final, fatal exit.
		os.Exit(1)
	}
}

// appServices holds the wired feature services.
type appServices struct {
	transactions *transactions.Service
	alerts       *alerts.Service
	rules        rules.Repository
}

// buildServices wires the feature modules over a connection pool, registering
// the alerts service as the transactions observer so that ingesting a
// transaction triggers rule evaluation and alert raising.
func buildServices(pool *pgxpool.Pool, logger *slog.Logger) appServices {
	rulesRepo := rulestore.New(pool)
	alertSvc := alerts.NewService(alertstore.New(pool), rulesRepo, logger)

	txSvc := transactions.NewService(
		txstore.New(pool),
		transactions.NewGenerator(time.Now().UnixNano()),
		logger,
	)
	txSvc.SetObserver(alertSvc)

	return appServices{transactions: txSvc, alerts: alertSvc, rules: rulesRepo}
}

// runSeed ingests synthetic transactions for local development and testing.
func runSeed(args []string) error {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	count := fs.Int("n", 50, "number of synthetic transactions to ingest")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(os.Getenv)
	logger := newLogger(cfg, err)
	if err != nil {
		logger.Error("invalid configuration", slog.String("error", err.Error()))
		return err
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to initialise database pool", slog.String("error", err.Error()))
		return err
	}
	defer pool.Close()

	svcs := buildServices(pool, logger)

	// Ensure the generic default rules exist so ingestion can raise alerts.
	if err := svcs.rules.EnsureDefaults(ctx); err != nil {
		logger.Error("failed to ensure default rules", slog.String("error", err.Error()))
		return err
	}

	persisted, err := svcs.transactions.Ingest(ctx, *count)
	if err != nil {
		logger.Error("seed failed", slog.Int("persisted", persisted), slog.String("error", err.Error()))
		return err
	}
	logger.Info("seed complete", slog.Int("persisted", persisted))
	return nil
}

// healthcheck performs a localhost liveness request against the configured port.
func healthcheck() error {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/health/live", port))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// run wires dependencies and owns the process lifecycle. It is separated from
// main so it can return errors instead of calling os.Exit, which keeps the
// shutdown path testable and linters happy.
func run() error {
	cfg, err := config.Load(os.Getenv)
	logger := newLogger(cfg, err)
	if err != nil {
		logger.Error("invalid configuration", slog.String("error", err.Error()))
		return err
	}

	logger.Info("starting finwatch api",
		slog.String("env", cfg.AppEnv),
		slog.String("addr", cfg.Addr()),
	)

	// The pool establishes connections lazily, so a database that is not yet up
	// does not block startup; readiness reports unavailable until it is.
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to initialise database pool", slog.String("error", err.Error()))
		return err
	}
	defer pool.Close()

	svcs := buildServices(pool, logger)

	router := httpserver.NewRouter(httpserver.RouterDeps{
		Logger: logger,
		Health: httpserver.NewHealthHandler(pool),
		Modules: []httpserver.RouteRegistrar{
			txhttp.NewHandler(svcs.transactions, logger),
			alerthttp.NewHandler(svcs.alerts, logger),
		},
	})

	srv := httpserver.New(httpserver.Options{
		Addr:              cfg.Addr(),
		Handler:           router,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	})

	// Trap termination signals to drive graceful shutdown.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			logger.Error("http server failed", slog.String("error", err.Error()))
			return err
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections",
			slog.Duration("timeout", cfg.ShutdownTimeout))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", slog.String("error", err.Error()))
		return err
	}
	logger.Info("shutdown complete")
	return nil
}

// newLogger builds a JSON structured logger. When configuration failed to load
// it falls back to info level so the failure itself is still logged.
func newLogger(cfg *config.Config, cfgErr error) *slog.Logger {
	level := slog.LevelInfo
	if cfgErr == nil {
		level = parseLevel(cfg.LogLevel)
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
