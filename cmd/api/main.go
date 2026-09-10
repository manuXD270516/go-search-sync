// Comando api arranca el servicio de búsqueda del catálogo farmacéutico.
package main

import (
	"context"
	"errors"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/manuXD270516/go-search-sync/internal/application/usecase"
	apihttp "github.com/manuXD270516/go-search-sync/internal/infrastructure/http"
	"github.com/manuXD270516/go-search-sync/internal/infrastructure/opensearch"
	"github.com/manuXD270516/go-search-sync/internal/platform/config"
)

func main() {
	cfg := config.Load()
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: nivel(cfg.LogLevel)}))

	engine := opensearch.New(cfg.SearchURL, cfg.SearchTimeout)
	cat := &usecase.Catalog{Engine: engine, Availability: engine}
	api := &apihttp.API{Catalog: cat, Log: log, Cache: apihttp.NewCache(cfg.CacheTTL)}

	srv := &stdhttp.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		log.Info("servicio de busqueda escuchando", "port", cfg.HTTPPort, "docs", "/docs", "search", cfg.SearchURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
			log.Error("fallo del servidor", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("apagando servicio")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("apagado forzado", "error", err)
	}
}

func nivel(s string) slog.Level {
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
