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

// ============================================================================
// MTTR Application — Express Bootstrap
// ----------------------------------------------------------------------------
// Wires together the middleware stack, mounts every route module, and starts
// the HTTP server + the daily aggregation cron job.
//
// Boot sequence:
//   1. Apply security middleware (helmet, CORS, rate-limit, body parser).
//   2. Install a per-request access log.
//   3. Mount routes (health → cases → mttr → admin → summary).
//   4. Install 404 + global error handlers.
//   5. Start listening, then start the cron job that recomputes MTTR daily.
// ============================================================================

const express = require('express');
const helmet = require('helmet');
const cors = require('cors');
const rateLimit = require('express-rate-limit');
const applicationConfig = require('./config');
const logger = require('./utils/logger');
const { authenticate, requireAdmin } = require('./middleware/auth');
const { correlationId } = require('./middleware/correlationId');
const { startAggregationJob } = require('./jobs/aggregationJob');

// ── Route modules ─────────────────────────────────────────────────────────
const casesRoutes = require('./routes/cases');
const mttrRoutes = require('./routes/mttr');
const adminRoutes = require('./routes/admin');
const healthRoutes = require('./routes/health');
const summaryRoutes = require('./routes/summary');

const expressApp = express();

// Trust the Choreo gateway that sits one hop in front of this container.
// Without this, req.ip resolves to the gateway's internal IP for every
// request — meaning the rate limiter counts against one global bucket
// instead of per real-client IP (from the gateway-set X-Forwarded-For
// header). `1` = trust exactly one proxy in the chain.
expressApp.set('trust proxy', 1);

// ─── Security & body parsing ──────────────────────────────────────────────

// helmet sets a sensible set of HTTP security headers with strict defaults.
expressApp.use(helmet());

// CORS: production restricts to an explicit allowlist (set via
// CORS_ALLOWED_ORIGINS as a comma-separated list); dev/test is wide open
// for easier iteration.
expressApp.use(cors({
    origin: applicationConfig.env === 'production'
        ? (process.env.CORS_ALLOWED_ORIGINS || '').split(',').filter(Boolean)
        : '*',
}));

// 10MB body limit accommodates a worst-case 5000-case bulk-import payload.
expressApp.use(express.json({ limit: '10mb' }));

// Correlation-ID: reuse the caller's X-Correlation-Id header if present
// (that's how the SN scheduled job's batch_id flows into our logs) or
// mint a fresh UUID. Installed before the access log so every line
// emitted below can quote req.correlationId.
expressApp.use(correlationId);

// ─── Rate limiting (applied to /api/* only) ───────────────────────────────

const apiRateLimiter = rateLimit({
    windowMs: applicationConfig.rateLimit.windowMs,
    max: applicationConfig.rateLimit.max,
    standardHeaders: true,    // emit RateLimit-* headers
    legacyHeaders: false,     // suppress X-RateLimit-* (deprecated)
    message: { error: 'Too many requests, please try again later.' },
});
expressApp.use('/api/', apiRateLimiter);

// ─── Per-request access log ───────────────────────────────────────────────

expressApp.use((req, res, next) => {
    const requestStartedAt = Date.now();
    res.on('finish', () => {
        logger.info(`${req.method} ${req.originalUrl} ${res.statusCode} ${Date.now() - requestStartedAt}ms`, {
            correlationId: req.correlationId,
        });
    });
    next();
});

// ─── Route mounting ───────────────────────────────────────────────────────
//
// Health is intentionally first and unauthenticated — load balancers,
// readiness probes, and the Docker HEALTHCHECK all need to hit it without
// credentials.

// Health (no auth — used by load balancers / Choreo probes)
expressApp.use('/api/v1/health', healthRoutes);

// Cases ingestion (authenticated — SN scheduled job)
expressApp.use('/api/v1/cases', authenticate, casesRoutes);

// MTTR query (authenticated — SN widgets)
expressApp.use('/api/v1/mttr', authenticate, mttrRoutes);

// Admin (authenticated + admin role required)
expressApp.use('/api/v1/admin', authenticate, requireAdmin, adminRoutes);

// Historical summaries (authenticated — quarterly trend charts)
expressApp.use('/api/v1/summary', authenticate, summaryRoutes);

// ─── 404 + global error handlers ──────────────────────────────────────────

expressApp.use((req, res) => {
    res.status(404).json({ error: 'Not found', correlation_id: req.correlationId });
});

// Any error thrown synchronously or passed to next(err) lands here.
// Returns a generic 500 to the client and logs the full stack for ops.
// The correlation ID is echoed in the response so a caller can quote it
// back when reporting an issue — no need to correlate by timestamp.
expressApp.use((err, req, res, next) => {
    logger.error('Unhandled error', {
        error: err.message,
        stack: err.stack,
        correlationId: req.correlationId,
    });
    res.status(500).json({ error: 'Internal server error', correlation_id: req.correlationId });
});

// ─── Start server + cron ──────────────────────────────────────────────────

expressApp.listen(applicationConfig.port, () => {
    logger.info(`MTTR App started on port ${applicationConfig.port} (${applicationConfig.env})`);

    // Kick off the daily aggregation cron once the HTTP server is up
    // so we don't risk computing while the DB pool is still warming.
    startAggregationJob();
});

module.exports = expressApp;
