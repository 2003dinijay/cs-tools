// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Command server serves the vendor webhook routes and health endpoints.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"sre-alert-edge-service/internal/auth"
	"sre-alert-edge-service/internal/config"
	"sre-alert-edge-service/internal/server"
	"sre-alert-edge-service/internal/vendors"
)

func main() {
	base := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("app", "sre-alert-edge-service")
	logger := base.With("component", "main")

	cfg, err := config.Load("")
	if err != nil {
		logger.Error("failed to load deployment config", "error", err)
		os.Exit(1)
	}
	envCfg, err := config.LoadEnv()
	if err != nil {
		logger.Error("failed to read environment", "error", err)
		os.Exit(1)
	}
	authn, err := auth.New(cfg.Auth.Mode)
	if err != nil {
		logger.Error("failed to initialise auth hook", "error", err)
		os.Exit(1)
	}
	if cfg.Auth.Mode == "none" {
		logger.Warn("auth.mode is \"none\": vendor routes are unauthenticated")
	}
	registry, err := vendors.New()
	if err != nil {
		logger.Error("failed to load vendor config", "error", err)
		os.Exit(1)
	}

	srv := server.New(server.Options{
		Logger:       base.With("component", "server"),
		Auth:         authn,
		Vendors:      registry.Names(),
		MaxBodyBytes: cfg.Server.MaxBodyBytes,
		ReadTimeout:  cfg.Server.ReadTimeout.Duration(),
		WriteTimeout: cfg.Server.WriteTimeout.Duration(),
	})
	httpSrv := srv.HTTPServer(":" + envCfg.Port)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server listening", "port", envCfg.Port)
		serveErr <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server exited unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining")
		// Restores default signal handling so a second Ctrl-C force-kills.
		stop()
		srv.StartDraining()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownGrace.Duration())
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}
}
