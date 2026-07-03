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
// /api/v1/health — Service Health Endpoint
// ----------------------------------------------------------------------------
// Returns a JSON snapshot of the service's runtime state. Used by:
//   • Choreo's load-balancer / readiness probes
//   • The Dockerfile HEALTHCHECK
//
// Intentionally not behind auth — health probes should never need a token.
// Returns:
//   200 + healthy snapshot when the DB responds.
//   503 + error message when the DB is unreachable.
// ============================================================================

const express = require('express');
const db = require('../config/database');
const { getLastCalculatedAt, getCacheCount } = require('../services/cacheService');
const logger = require('../utils/logger');

const router = express.Router();

/**
 * GET /api/v1/health
 * Returns DB connectivity + ingestion / aggregation freshness indicators.
 */
router.get('/', async (req, res) => {
    try {
        // Liveness ping. If this throws, the whole endpoint fails fast.
        const dbServerTime = await db.healthCheck();

        // Run the diagnostic queries in parallel — none depend on each other
        // and the response time is dominated by the slowest one.
        const [
            totalCasesQuery,
            lastCalculatedAt,
            cacheRowCount,
            lastIngestionQuery,
        ] = await Promise.all([
            db.query('SELECT COUNT(*) AS count FROM case_events'),
            getLastCalculatedAt(),
            getCacheCount(),
            db.query('SELECT MAX(ingested_at) AS last_ingestion_at FROM ingestion_log'),
        ]);

        return res.json({
            status: 'healthy',
            db_time: dbServerTime,
            total_cases: parseInt(totalCasesQuery.rows[0].count, 10),
            last_ingestion_at: lastIngestionQuery.rows[0].last_ingestion_at || null,
            last_aggregation_at: lastCalculatedAt,
            cache_entries: cacheRowCount,
        });
    } catch (healthCheckError) {
        // Any failure here means the DB is down or unreachable — return
        // 503 so external probes can react (restart the pod, alert, etc.).
        //
        // The response body is intentionally generic: /health is
        // unauthenticated so leaking driver text, hostnames, or schema
        // names in the JSON would disclose internal detail to anyone
        // who can reach the endpoint. Full context lives in the log.
        logger.error('Health check failed', {
            error: healthCheckError.message,
            stack: healthCheckError.stack,
        });
        return res.status(503).json({
            status: 'unhealthy',
            error: 'Service temporarily unavailable',
        });
    }
});

module.exports = router;
