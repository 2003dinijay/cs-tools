// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
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

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/service"

	applog "github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/log"
)

// buildMux registers the routes of the enabled flows. A disabled flow's paths
// are left unregistered, so they answer 404.
func buildMux(cfg *config.Config, h *handler.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	if cfg.SFTPChannelEnabled {
		mux.HandleFunc("/prelogin-hook", h.PreLoginHook)
		mux.HandleFunc("/auth-hook", h.AuthHandler)
	}
	if cfg.WebAttachmentEnabled {
		mux.HandleFunc("/external-auth-hook", h.ExternalAuthHook)
	}
	return mux
}

// initJWTAuth builds the JWT validator for the web-attachment flow via newJWT.
// When ENABLED_FLOWS explicitly names the flow, a construction error is
// returned so the caller can refuse to start. When ENABLED_FLOWS is unset the
// error is only logged and a nil validator is returned, leaving
// /external-auth-hook answering 503 (the behavior before ENABLED_FLOWS existed).
func initJWTAuth(
	cfg *config.Config,
	logger *applog.AppLogger,
	newJWT func(*config.Config, *applog.AppLogger) (*service.JWTAuthService, error),
) (*service.JWTAuthService, error) {
	svc, err := newJWT(cfg, logger)
	if err == nil {
		return svc, nil
	}
	if cfg.FlowsExplicit {
		return nil, err
	}
	logger.Warn("External-auth hook (web attachment access path) is disabled: %v", err)
	return nil, nil
}

// The entry point for the SFTPGo Authentication Service.
func main() {
	// 1. Load configuration from environment variables
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("FATAL: Failed to load configuration: %v", err)
	}

	// 2. Initialize the custom logger
	logger := applog.NewAppLogger(cfg.LogLevel)

	logger.Info("Enabled flows: web-attachment=%t sftp-channel=%t", cfg.WebAttachmentEnabled, cfg.SFTPChannelEnabled)

	// 3. Initialize the dependencies of each enabled flow. A disabled flow's
	// dependencies are never built, so it needs none of their configuration
	// (and, for the sftp-channel flow, no reachable database).
	var (
		dbService           *service.DBService
		idpService          *service.IdPService
		sftpgoService       *service.SFTPGoService
		subscriptionService *service.SubscriptionService
		jwtAuthService      *service.JWTAuthService
	)

	if cfg.SFTPChannelEnabled {
		dbService, err = service.NewDBService(cfg, logger)
		if err != nil {
			logger.Fatal("Failed to initialize database: %v", err)
		}
		idpService = service.NewIdPService(cfg, logger)
		sftpgoService = service.NewSFTPGoService(cfg, logger)
		subscriptionService = service.NewSubscriptionService(cfg, logger)
	}

	// The JWT validator backs /external-auth-hook (web attachment access path).
	// With ENABLED_FLOWS unset, a validator that cannot be built does not stop
	// the service: /external-auth-hook fails closed with 503 per request. With
	// ENABLED_FLOWS set, it is fatal. An unreachable JWKS host is not an error
	// returned here: the key set refreshes in the background.
	if cfg.WebAttachmentEnabled {
		jwtAuthService, err = initJWTAuth(cfg, logger, service.NewJWTAuthService)
		if err != nil {
			logger.Fatal("Failed to initialize the JWT validator for the web-attachment flow: %v", err)
		}
	}

	// 4. Initialize the HTTP handler, injecting all dependencies
	h := handler.NewHandler(cfg, logger, dbService, idpService, sftpgoService, subscriptionService, jwtAuthService)

	// 5. Set up routes for the enabled flows
	mux := buildMux(cfg, h)

	// 6. Start the server with graceful shutdown support
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.IdleTimeout) * time.Second,
	}

	// Channel to listen for shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("SFTPGo Hooks Server listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-stop

	logger.Info("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited gracefully")
}
