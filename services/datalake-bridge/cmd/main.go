// datalake-bridge sincroniza las MÁQUINAS (grupo MAQ de D365) desde el Link to
// Fabric hacia Odoo, por número de serie. Es un servicio autónomo, aparte de
// synapse-bridge (que sigue con refacciones/accesorios): un diff auto-contenido
// que solo escribe en Odoo lo que cambió.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"datalake-bridge/internal/config"
	"datalake-bridge/internal/datalake"
	"datalake-bridge/internal/odoo"
	syncengine "datalake-bridge/internal/sync"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		log.Error("configuración inválida", "error", err.Error())
		os.Exit(1)
	}

	reader, err := datalake.Open(cfg.FabricJDBCURL, cfg.FabricClientID, cfg.FabricSecret, cfg.AzureTenantID)
	if err != nil {
		log.Error("no se pudo abrir el datalake", "error", err.Error())
		os.Exit(1)
	}
	defer reader.Close()

	oc := odoo.NewClient(cfg.OdooBaseURL, cfg.OdooAPIKey, cfg.OdooDatabase)
	engine := syncengine.New(reader, oc, cfg.MachineCategory, cfg.DryRun, log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Sonda de salud para Container Apps (liveness). Marca listo tras validar el
	// datalake; el sync corre en segundo plano.
	var ready atomic.Bool
	go serveHealth(ctx, log, &ready)

	pingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	if err := reader.Ping(pingCtx); err != nil {
		cancel()
		log.Error("ping al datalake falló", "error", err.Error())
		os.Exit(1)
	}
	cancel()
	ready.Store(true)

	runOnce := func() {
		runCtx, c := context.WithTimeout(ctx, 15*time.Minute)
		defer c()
		if _, err := engine.Run(runCtx); err != nil {
			log.Error("pasada de sync falló", "error", err.Error())
		}
	}

	// Modo una-pasada: sin intervalo, corre y sale (útil para un Job programado).
	if cfg.Interval == 0 {
		log.Info("modo pasada única", "dry_run", cfg.DryRun)
		runOnce()
		return
	}

	// Modo servicio: pasada periódica.
	log.Info("modo periódico", "intervalo", cfg.Interval.String(), "dry_run", cfg.DryRun)
	if cfg.RunOnAtStart {
		runOnce()
	}
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("apagando datalake-bridge")
			return
		case <-ticker.C:
			runOnce()
		}
	}
}

func serveHealth(ctx context.Context, log *slog.Logger, ready *atomic.Bool) {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if !ready.Load() {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	srv := &http.Server{Addr: ":" + port, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("servidor de salud terminó", "error", err.Error())
	}
}
