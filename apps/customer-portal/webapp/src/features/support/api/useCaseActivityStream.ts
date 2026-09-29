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

import { useEffect } from "react";
import EventSourcePolyfill from "@sanity/eventsource";
import { useAsgardeo } from "@asgardeo/react";
import { useQueryClient } from "@tanstack/react-query";
import { apiConfig } from "@config/apiConfig";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useLogger } from "@hooks/useLogger";

/** Base delay before the first reconnect attempt after the stream errors out or drops. */
const RECONNECT_BASE_DELAY_MS = 3_000;
/** Reconnect delay never grows past this, no matter how many consecutive failures. */
const RECONNECT_MAX_DELAY_MS = 30_000;

/**
 * Exponential backoff with full jitter (attempt 0 is a random delay in
 * [0, base), attempt 1 in [0, base*2), ... capped at max) — a sustained
 * backend outage or misconfiguration would otherwise have every open
 * case-detail tab retry in lockstep every RECONNECT_BASE_DELAY_MS forever,
 * hammering the endpoint indefinitely instead of backing off.
 */
function reconnectDelay(attempt: number): number {
  const capped = Math.min(RECONNECT_MAX_DELAY_MS, RECONNECT_BASE_DELAY_MS * 2 ** attempt);
  return Math.random() * capped;
}

/**
 * Opens a live Server-Sent Events connection to
 * customer-portal-activity-stream-service's
 * `GET /cases/{id}/activities/stream` and invalidates the case's comments and
 * details queries whenever it emits a `case_updated` event, so a new comment
 * or status change shows up without the viewer having to wait out those
 * queries' own staleTime or refresh manually.
 *
 * Uses `@sanity/eventsource` rather than the browser's native `EventSource`
 * because native EventSource cannot set custom headers — it only supports
 * cookies/query params for auth.
 *
 * All three headers carry the Asgardeo ID token, and all three are load-bearing
 * (verified against the deployed staging endpoint):
 *   - `Authorization` is what gets past Choreo's gateway, which enforces its
 *     own OAuth2 check on this operation; without it the gateway answers 401
 *     `900901` before the request ever reaches the service.
 *   - `x-jwt-assertion` is what the service's own `middleware.Auth` reads and
 *     validates. The gateway forwards it untouched rather than minting its
 *     own, which is why the service validates it against Asgardeo directly.
 *   - `x-user-id-token` is forwarded upstream to entity-service for the
 *     per-case ACL check that authorizes the subscription.
 * This mirrors useAuthApiClient, which likewise sends the ID token as both
 * `Authorization` and `x-user-id-token` for every other backend call.
 *
 * Headers are fixed at construction time, so they can't be refreshed on the
 * library's own built-in reconnect — a token that expires mid-connection
 * would otherwise have the polyfill retry forever with the same stale header.
 * Instead, `error` closes the current connection and this hook opens a fresh
 * one with a newly-fetched token after an exponentially backed-off delay.
 *
 * A no-op when `caseId` is unset, `apiConfig.streamEnabled` is false (the
 * feature's master switch, `CUSTOMER_PORTAL_STREAM_ENABLED` — defaults off),
 * or `apiConfig.streamUrl` isn't configured (Event Hub — and therefore this
 * endpoint — is optional on the backend); callers fall back to the
 * comments/details queries' own staleTime.
 */
export function useCaseActivityStream(caseId: string | undefined): void {
  const queryClient = useQueryClient();
  const { getIdToken } = useAsgardeo();
  const logger = useLogger();

  useEffect(() => {
    if (!caseId || !apiConfig.streamEnabled || !apiConfig.streamUrl) return;

    let cancelled = false;
    let source: EventSourcePolyfill | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;
    let hasConnected = false;

    const invalidateCaseQueries = (): void => {
      // Invalidated by key prefix rather than the queries' full keys, which
      // also carry the project id this hook isn't given — the same thing
      // usePostComment already does after posting a comment.
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CASE_COMMENTS],
      });
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CASE_DETAILS],
      });
    };

    const scheduleReconnect = (): void => {
      const delay = reconnectDelay(attempt);
      attempt += 1;
      reconnectTimer = setTimeout(() => void connect(), delay);
    };

    const connect = async (): Promise<void> => {
      let idToken: string | undefined;
      try {
        idToken = await getIdToken();
      } catch (error) {
        logger.debug(
          "[case-activity-stream] failed to get ID token",
          error instanceof Error ? error.message : "Unknown token error",
        );
      }
      if (cancelled) return;
      if (!idToken) {
        scheduleReconnect();
        return;
      }

      const url = `${apiConfig.streamUrl}/cases/${encodeURIComponent(caseId)}/activities/stream`;
      source = new EventSourcePolyfill(url, {
        headers: {
          Authorization: `Bearer ${idToken}`,
          "x-jwt-assertion": idToken,
          "x-user-id-token": idToken,
        },
      });

      source.addEventListener("open", () => {
        // A successful connection resets the backoff — only *consecutive*
        // failures should back off, not the cumulative count over the
        // component's whole lifetime.
        attempt = 0;

        // The stream only ever carries events published while a connection
        // is registered — the service's hub hands a new subscriber a fresh
        // channel and its consumer reads from the latest offset, so nothing
        // missed during a drop is replayed. Refetch on every *re*connection
        // to close that gap, which the deployment's own connection lifetime
        // makes a routine occurrence rather than an edge case. Skipped on the
        // first connection, where the queries have just loaded anyway.
        if (hasConnected) {
          invalidateCaseQueries();
        }
        hasConnected = true;
      });

      source.addEventListener("case_updated", invalidateCaseQueries);

      source.addEventListener("error", () => {
        logger.debug("[case-activity-stream] connection error, reconnecting");
        source?.close();
        if (!cancelled) {
          scheduleReconnect();
        }
      });
    };

    void connect();

    return () => {
      cancelled = true;
      clearTimeout(reconnectTimer);
      source?.close();
    };
  }, [caseId, queryClient, getIdToken, logger]);
}
