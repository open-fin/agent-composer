// Command composer-server runs the agent-composer REST API.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/open-fin/agent-composer/internal/api"
	"github.com/open-fin/agent-composer/internal/config"
	"github.com/open-fin/agent-composer/internal/engine"
	"github.com/open-fin/agent-composer/internal/llm"
	"github.com/open-fin/agent-composer/internal/store"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to the configuration file")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	if err := run(*configPath, log); err != nil {
		log.Error("composer-server exited", "error", err)
		os.Exit(1)
	}
}

func run(configPath string, log *slog.Logger) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dataStore, storeKind, err := openStore(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer dataStore.Close()

	enricher := buildEnricher(cfg, log)
	log.Info("starting composer-server",
		"addr", cfg.HTTP.Addr, "store", storeKind, "enricher", enricher.Name())

	composer := engine.New(engine.Options{Store: dataStore, Enricher: enricher, Logger: log})
	server := api.NewServer(api.ServerOptions{
		Engine:    composer,
		Store:     dataStore,
		StoreKind: storeKind,
		Config:    cfg,
		Logger:    log,
	})

	httpServer := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      server.Handler(),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

// openStore selects the durable store when a DSN is configured, and the in-memory one
// otherwise. Running without a database is a supported mode: it is how the demo can be
// shown from a single binary.
func openStore(ctx context.Context, cfg config.Config, log *slog.Logger) (store.Store, string, error) {
	if !cfg.UsesPostgres() {
		log.Warn("no database configured, using the in-memory store; data is lost on restart")
		return store.NewMemory(), "memory", nil
	}
	postgres, err := store.NewPostgres(ctx, cfg.Database.DSN, cfg.Database.MaxConns,
		cfg.Database.StartupAttempts, cfg.Database.MigrateOnStart, log)
	if err != nil {
		return nil, "", err
	}
	return postgres, "postgres", nil
}

// buildEnricher returns the configured LLM enricher, or the deterministic mock. The
// mock is not a degraded mode: it is the default, and it is what makes the demo
// reproducible.
func buildEnricher(cfg config.Config, log *slog.Logger) llm.Enricher {
	if !cfg.LLM.Enabled {
		return llm.NewMockEnricher()
	}
	client := llm.NewOpenAICompatible(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.Timeout)
	return llm.NewClientEnricher(client, log)
}
