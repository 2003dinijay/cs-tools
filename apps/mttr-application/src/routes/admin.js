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
// /api/v1/admin — Operator-Only Administration Endpoints
// ----------------------------------------------------------------------------
// Every route here is mounted behind both `authenticate` AND `requireAdmin`
// in src/index.js, so callers must present a JWT carrying the `mttr-admin`
// role/group claim.
//
// Routes:
//   POST /api/v1/admin/cache/reset        – force MTTR recomputation
//   GET  /api/v1/admin/ingestion-logs     – inspect recent batch history
//   POST /api/v1/admin/retention/run      – manually trigger retention
//
// These are intended for operators using `curl` or Postman — not for
// automated callers.
// ============================================================================

const express = require('express');
const { query, validationResult } = require('express-validator');
const { runFullAggregation, runAggregationForType, DIMENSIONS } = require('../services/aggregationService');
const { getIngestionLogs } = require('../services/ingestionService');
const { runRetention } = require('../services/retentionService');
const logger = require('../utils/logger');

const router = express.Router();

// Same whitelist used by /mttr. Sourced from DIMENSIONS so adding a new
// aggregation dimension automatically becomes resettable here too.
const VALID_DIMENSION_TYPES = Object.keys(DIMENSIONS);

/**
 * POST /api/v1/admin/cache/reset[?type=<dim>]
 *
 * Recomputes cached aggregations. Without `?type=…` rebuilds all
 * dimensions in one transaction; with it, rebuilds just that dimension
 * (much faster, useful when only one SN widget looks stale).
 */
router.post('/cache/reset', async (req, res) => {
    const requestedDimensionType = req.query.type;

    try {
        // Branch 1: targeted reset for a single dimension.
        if (requestedDimensionType) {
            if (!VALID_DIMENSION_TYPES.includes(requestedDimensionType)) {
                return res.status(400).json({
                    error: `Invalid type. Must be one of: ${VALID_DIMENSION_TYPES.join(', ')}`,
                });
            }
            const result = await runAggregationForType(requestedDimensionType, req.correlationId);
            return res.json({
                status: 'ok',
                recalculated: [requestedDimensionType],
                ...result,
            });
        }

        // Branch 2: full rebuild of every dimension.
        const fullResult = await runFullAggregation(req.correlationId);
        return res.json({
            status: 'ok',
            recalculated: Object.keys(DIMENSIONS),
            ...fullResult,
        });
    } catch (resetError) {
        logger.error('Cache reset error', {
            correlationId: req.correlationId,
            error: resetError.message,
        });
        return res.status(500).json({ error: 'Aggregation failed', correlation_id: req.correlationId });
    }
});

/**
 * GET /api/v1/admin/ingestion-logs?limit=<n>
 *
 * Returns the most recent ingestion-log rows (newest first), capped at
 * `limit` (1–200, default 50).
 */
router.get('/ingestion-logs', async (req, res) => {
    try {
        // Clamp at 200 so a runaway client can't ask for the entire log.
        const requestedLimit = Math.min(parseInt(req.query.limit, 10) || 50, 200);
        const ingestionLogRows = await getIngestionLogs(requestedLimit);
        return res.json({ logs: ingestionLogRows });
    } catch (logsQueryError) {
        logger.error('Ingestion logs error', {
            correlationId: req.correlationId,
            error: logsQueryError.message,
        });
        return res.status(500).json({ error: 'Internal server error', correlation_id: req.correlationId });
    }
});

/**
 * POST /api/v1/admin/retention/run
 *
 * Manually triggers the same retention job that the daily cron runs.
 * Useful right after a large historical import, or when adjusting the
 * retention window and wanting to apply it immediately.
 */
router.post('/retention/run', async (req, res) => {
    try {
        logger.info('Admin: Manual retention job triggered', { correlationId: req.correlationId });
        const retentionResult = await runRetention(req.correlationId);
        return res.json({
            status: 'ok',
            message: 'Retention job completed',
            ...retentionResult,
        });
    } catch (retentionError) {
        logger.error('Admin retention job error', {
            correlationId: req.correlationId,
            error: retentionError.message,
        });
        return res.status(500).json({ error: 'Retention job failed', correlation_id: req.correlationId });
    }
});

module.exports = router;
