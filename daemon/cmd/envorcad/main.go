// envorcad is the Envorca daemon: the single owner of local infrastructure
// state and logic. It runs as a user-session background process (ADR-0002)
// and serves clients over a user-restricted local IPC channel (ADR-0003).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"google.golang.org/grpc"
	grpchealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	envorcav1 "envorca.dev/envorca/api/gen/go/envorca/v1"
	"envorca.dev/envorca/api/ipc"
	daemonapi "envorca.dev/envorca/daemon/internal/api"
	"envorca.dev/envorca/daemon/internal/config"
	"envorca.dev/envorca/daemon/internal/diagnosis"
	"envorca.dev/envorca/daemon/internal/events"
	"envorca.dev/envorca/daemon/internal/logging"
	"envorca.dev/envorca/daemon/internal/recovery"
	"envorca.dev/envorca/daemon/internal/state"
	"envorca.dev/envorca/daemon/internal/version"
	"envorca.dev/envorca/daemon/internal/wsl"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "envorcad:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "path to config.yaml (default: none)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Version)
		return nil
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	level, err := logging.Level(cfg.LogLevel)
	if err != nil {
		return err
	}
	logFile, err := logging.OpenFile(cfg.LogDir())
	if err != nil {
		return err
	}
	defer logFile.Close()
	logger := logging.New(level, io.MultiWriter(logFile, os.Stdout))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := events.New(256)
	bus.Publish(events.Event{Level: "info", Component: "daemon", Name: "starting",
		Fields: map[string]any{"version": version.Version, "state_dir": cfg.StateDir}})

	db, err := state.Open(cfg.DBPath())
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate state: %w", err)
	}
	if v, err := db.SchemaVersion(ctx); err == nil {
		logger.Info("state ready", "schema_version", v)
	}

	reg, err := diagnosis.Registry(diagnosis.Options{})
	if err != nil {
		return fmt.Errorf("diagnosis registry: %w", err)
	}

	endpoint := cfg.Endpoint()
	ln, err := ipc.Listen(endpoint)
	if err != nil {
		return fmt.Errorf("listen %s: %w", endpoint, err)
	}

	grpcServer := grpc.NewServer()
	rec := recovery.New(wsl.DefaultRunner())
	daemonServer := daemonapi.New(logger, bus, reg, rec, db, endpoint, cancel)
	envorcav1.RegisterDaemonServer(grpcServer, daemonServer)
	hs := grpchealth.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	hs.SetServingStatus("envorca.v1.Daemon", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, hs)

	pidFile := filepath.Join(cfg.RuntimeDir(), "envorca.pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	defer os.Remove(pidFile)

	serveErr := make(chan error, 1)
	go func() {
		bus.Publish(events.Event{Level: "info", Component: "daemon", Name: "listening",
			Fields: map[string]any{"endpoint": endpoint}})
		serveErr <- grpcServer.Serve(ln)
	}()

	logger.Info("daemon started", "version", version.Version, "endpoint", endpoint, "state_dir", cfg.StateDir)

	var stopOnce sync.Once
	stop := func(reason string) {
		stopOnce.Do(func() {
			bus.Publish(events.Event{Level: "info", Component: "daemon", Name: "stopping", Fields: map[string]any{"reason": reason}})
			grpcServer.GracefulStop()
			ipc.Cleanup(endpoint)
			db.Close()
			bus.Publish(events.Event{Level: "info", Component: "daemon", Name: "stopped", Fields: map[string]any{"reason": reason}})
		})
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-serveErr:
		stop("serve_failure")
		return fmt.Errorf("grpc serve: %w", err)
	case sig := <-sigCh:
		logger.Info("received signal", "signal", sig.String())
		stop("signal")
		return nil
	case <-ctx.Done():
		stop("shutdown_request")
		return nil
	}
}
