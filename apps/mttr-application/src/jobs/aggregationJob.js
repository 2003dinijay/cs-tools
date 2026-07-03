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
// Daily Aggregation + Retention Cron Job
// ----------------------------------------------------------------------------
// Started once at application boot (from src/index.js) and registers a
// node-cron schedule based on AGGREGATION_CRON (default "0 0 * * *" = daily
// at midnight).
//
// On every tick it performs three steps, in order:
//
//   1. runFullAggregation()  – recompute MTTR for every dimension across
//                              the rolling 12-month window and UPSERT into
//                              `mttr_cache`.
//
//   2. purgeStaleCache()     – delete any `mttr_cache` rows that weren't
//                              produced by the just-completed run (handles
//                              renamed/deleted dimension labels).
//
//   3. runRetention()        – purge old ingestion-log rows, summarise
//                              completed quarters into `case_events_summary`,
//                              and delete raw `case_events` rows older
//                              than the retention cutoff.
//
// All three steps are best-effort: any uncaught error is logged and the
// next tick simply tries again. Failures here do NOT take the API down.
// ============================================================================

const cron = require('node-cron');
const { v4: uuidv4 } = require('uuid');
const applicationConfig = require('../config');
const { runFullAggregation } = require('../services/aggregationService');
const { purgeStaleCache, runRetention } = require('../services/retentionService');
const db = require('../config/database');
const logger = require('../utils/logger');

/**
 * Register the daily cron. Idempotent — call once at boot.
 */
function startAggregationJob() {
    const cronExpression = applicationConfig.aggregation.cron;
    logger.info(`Scheduling aggregation job with cron: ${cronExpression}`);

    cron.schedule(cronExpression, async () => {
        // Mint a per-run correlation ID so every log line emitted by
        // this tick — and by the aggregation / retention functions it
        // calls into — can be filtered together. The "cron_" prefix
        // makes it obvious in logs that the trace wasn't request-driven.
        const correlationId = 'cron_' + uuidv4();
        try {
            // ── Step 1: Recompute every MTTR aggregation ───────────────
            logger.info('Aggregation cron job triggered', { correlationId });
            const aggregationResult = await runFullAggregation(correlationId);
            logger.info('Aggregation cron job completed', { correlationId, ...aggregationResult });

            // ── Step 2: Drop cache rows that aren't part of the latest run
            // We need a dedicated client because purgeStaleCache runs its
            // single DELETE inside the caller's transaction context.
            const dbClient = await db.getClient();
            try {
                await purgeStaleCache(dbClient, correlationId);
            } finally {
                dbClient.release();
            }

            // ── Step 3: Run retention (ingestion logs + summarise/delete)
            const retentionResult = await runRetention(correlationId);
            logger.info('Retention job completed', { correlationId, ...retentionResult });
        } catch (cronError) {
            // Swallow + log so the cron stays alive for the next tick.
            logger.error('Aggregation/retention cron job failed', {
                correlationId,
                error: cronError.message,
                stack: cronError.stack,
            });
        }
    });
}

module.exports = { startAggregationJob };
