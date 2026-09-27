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

package main

import (
	"context"
	"log/slog"
	"net/http"
)

// drainer is the part of the server shutdown needs; *server.Server implements it.
type drainer interface {
	StartDraining()
}

// closer is the allocator's shutdown; *allocator.Allocator implements it.
type closer interface {
	Close(ctx context.Context) error
}

// shutdown runs the SIGTERM sequence within ctx (server.shutdown_grace):
//  1. /healthz answers 503 so the platform stops routing here;
//  2. the HTTP server stops accepting and waits for in-flight requests, which are themselves
//     waiting on the allocator, so their alerts are still claimed and written;
//  3. the allocator closes its queue, claims whatever is left, and waits for the writers, so no
//     claimed id is left without a row;
//  4. after, in order: the last wake-up to alerts-core and any pending Chat cards.
func shutdown(ctx context.Context, logger *slog.Logger, srv drainer, httpSrv *http.Server, alloc closer, after ...func(context.Context)) {
	srv.StartDraining()
	if err := httpSrv.Shutdown(ctx); err != nil {
		logger.Error("http shutdown incomplete", "error", err)
	}
	if err := alloc.Close(ctx); err != nil {
		logger.Error("allocator did not drain within shutdown_grace; claimed ids may be left without rows", "error", err)
	}
	for _, f := range after {
		f(ctx)
	}
	logger.Info("shutdown complete")
}
