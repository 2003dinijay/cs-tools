#!/usr/bin/env bash
# Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
#
# The SRE escalation ladder, end to end, on the real clock.
#
# Unlike trigger-sre-escalation.sh, which drives the engine in-process on a
# compressed clock, this goes through the running system: the event is put on
# the local Kafka topic by entity-service's own publisher, consumed by the
# running csm-notification-service container (both engines, their own consumer
# groups, Redis, the ticker), resolved against the Team Schedule, and stopped
# by a real incident.assigned / incident.acknowledged / incident.comment_added
# on the same topic. One ladder minute is one real minute: L2 comes 5 minutes
# after L1. Nothing is dialled: escalation.yaml runs both ladders on the log
# channel.
#
# Only incident creation is skipped -- POST /incidents is 503 on the local
# Postgres data source -- so the publisher stands in for entity-service.
#
# Usage:
#   sre-e2e.sh up                      build, start, roster L1/L2/L3 for now
#   sre-e2e.sh create [-t team] [-p priority] [-x contactType]
#                                      publish incident.created (default apollo, HIGH)
#   sre-e2e.sh ack assign|status|comment [incident]
#                                      publish a stop gesture (default: the last incident)
#   sre-e2e.sh watch [incident]        follow the ladder in the service's log
#   sre-e2e.sh status [incident]       the ladder's state in Redis, both ladders
#   sre-e2e.sh oncall                  who holds each SRE tier right now
#   sre-e2e.sh down                    remove the test rota, restore the base stack
set -euo pipefail

cd "$(dirname "$0")/../.."
COMPOSE=(docker compose -f docker-compose.yml -f scripts/csm-compose/sre-e2e.compose.yml)
STATE="${TMPDIR:-/tmp}/sre-e2e-last-incident"
PSQL=(docker compose exec -T postgres psql -U postgres -d csm_platform -v ON_ERROR_STOP=1)

last_incident() { [[ -s "$STATE" ]] && cat "$STATE" || { echo "no incident yet: run '$0 create' first, or pass one" >&2; exit 1; }; }

build_publisher() {
  local arch
  arch="$(docker compose exec -T entity-service uname -m | tr -d '\r' | sed 's/aarch64/arm64/;s/x86_64/amd64/')"
  ( cd entity-service && GOOS=linux GOARCH="$arch" go build -o "${TMPDIR:-/tmp}/publish-incident" ./cmd/publish-incident )
  docker compose cp "${TMPDIR:-/tmp}/publish-incident" entity-service:/tmp/publish-incident >/dev/null
}

publish() { docker compose exec -T entity-service /tmp/publish-incident -broker kafka:9094 -topic case-events "$@"; }

oncall() {
  "${PSQL[@]}" -c "
    SELECT a.team_key AS team, coalesce(a.tier::text, s.tier::text) AS tier, s.code AS window, u.name AS engineer,
           to_char(a.ends_at AT TIME ZONE 'Asia/Kolkata', 'HH24:MI') AS until_ist, coalesce(a.note,'') AS note
      FROM team_schedule_assignment a
      JOIN team_schedule_shift s ON s.id = a.shift_id
      JOIN \"user\" u ON u.id = a.user_id
     WHERE s.family = 'SRE' AND s.is_escalation
       AND tstzrange(a.starts_at, a.ends_at, '[)') @> now()
       AND coalesce(a.tier::text, s.tier::text) IS NOT NULL
     ORDER BY 2, 1"
}

# Roster Apollo on every SRE tier of the escalation window live right now, for
# the tiers the seed leaves empty, with an engineer who is free (no absence that
# day, nothing else overlapping). Tagged QA-SRE-E2E; 'down' removes it.
roster_now() {
  "${PSQL[@]}" <<'SQL' >/dev/null
DO $$
DECLARE
  t text; w record; who uuid; apollo uuid;
BEGIN
  SELECT id INTO apollo FROM team WHERE key = 'apollo';
  FOREACH t IN ARRAY ARRAY['L1','L2','L3'] LOOP
    IF EXISTS (SELECT 1 FROM team_schedule_assignment a JOIN team_schedule_shift s ON s.id = a.shift_id
                WHERE a.team_key = 'apollo' AND s.family = 'SRE' AND s.is_escalation
                  AND coalesce(a.tier::text, s.tier::text) = t
                  AND tstzrange(a.starts_at, a.ends_at, '[)') @> now()) THEN
      CONTINUE;
    END IF;
    -- The live escalation window that can hold this tier: the L1 window for L1
    -- when there is one, otherwise the zone's open-tier window.
    SELECT s.id AS shift_id, s.zone_id, d.day,
           (d.day + make_interval(mins => s.start_minute)) AT TIME ZONE 'Asia/Kolkata' AS starts_at,
           (d.day + make_interval(mins => s.end_minute))   AT TIME ZONE 'Asia/Kolkata' AS ends_at
      INTO w
      FROM team_schedule_shift s,
           LATERAL (VALUES ((now() AT TIME ZONE 'Asia/Kolkata')::date),
                           ((now() AT TIME ZONE 'Asia/Kolkata')::date - 1)) d(day)
     WHERE s.family = 'SRE' AND s.is_escalation
       AND (s.tier IS NULL OR s.tier::text = t)
       AND (s.day_scope::text = 'ANY'
            OR (s.day_scope::text = 'WEEKDAY') = (extract(isodow FROM d.day) < 6))
       AND now() >= (d.day + make_interval(mins => s.start_minute)) AT TIME ZONE 'Asia/Kolkata'
       AND now() <  (d.day + make_interval(mins => s.end_minute))   AT TIME ZONE 'Asia/Kolkata'
     ORDER BY (s.tier IS NULL) -- a fixed-tier window first
     LIMIT 1;
    IF w IS NULL THEN
      RAISE NOTICE 'no live SRE window can hold %', t;
      CONTINUE;
    END IF;
    SELECT m.user_id INTO who FROM team_member m
     WHERE m.team_id = apollo AND m.role = 'engineer'
       AND NOT EXISTS (SELECT 1 FROM team_schedule_absence b WHERE b.user_id = m.user_id
                        AND b.starts_on <= w.day AND (b.ends_on IS NULL OR b.ends_on >= w.day))
       AND NOT EXISTS (SELECT 1 FROM team_schedule_assignment r WHERE r.user_id = m.user_id
                        AND tstzrange(r.starts_at, r.ends_at, '[)') && tstzrange(w.starts_at, w.ends_at, '[)'))
     ORDER BY m.user_id LIMIT 1;
    IF who IS NULL THEN
      RAISE NOTICE 'no free Apollo engineer for %', t;
      CONTINUE;
    END IF;
    INSERT INTO team_schedule_assignment
           (user_id, team_id, team_key, shift_id, zone_id, tier, rota_date, starts_at, ends_at,
            is_on_call, source, note, is_rotation, created_by, updated_by)
    VALUES (who, apollo, 'apollo', w.shift_id, w.zone_id,
            CASE WHEN (SELECT tier FROM team_schedule_shift WHERE id = w.shift_id) IS NULL THEN t::team_schedule_tier_enum END,
            w.day, w.starts_at, w.ends_at, TRUE, 'MANUAL', 'QA-SRE-E2E', TRUE, 'qa-sre-e2e', 'qa-sre-e2e');
  END LOOP;
END $$;
SQL
}

cmd="${1:-}"; shift || true
case "$cmd" in
  up)
    echo "==> building entity-service, the notification service and the gateway shim from this branch"
    "${COMPOSE[@]}" build entity-service csm-notification-service entity-gateway-shim >/dev/null
    echo "==> starting them with the SRE end-to-end override"
    "${COMPOSE[@]}" up -d entity-service entity-gateway-shim csm-notification-service mock-oidc redis kafka >/dev/null
    for _ in $(seq 60); do
      docker compose logs csm-notification-service --since 2m 2>/dev/null | grep -q 'ladder=escalation-sre' && break
      sleep 2
    done
    docker compose logs csm-notification-service --since 3m 2>/dev/null \
      | grep -E 'Team Schedule|incident call escalation is enabled' | sed 's/^/    /' | tail -3
    echo "==> rostering Apollo L1/L2/L3 on the SRE window live now (QA-SRE-E2E)"
    roster_now
    oncall
    echo "==> ready. Next: $0 create"
    ;;
  create)
    TEAM=apollo PRIORITY=HIGH CONTACT=""
    while getopts ":t:p:x:" opt; do
      case $opt in t) TEAM=$OPTARG ;; p) PRIORITY=$OPTARG ;; x) CONTACT=$OPTARG ;; *) echo "create [-t team] [-p priority] [-x contactType]" >&2; exit 2 ;; esac
    done
    [[ "$TEAM" == none ]] && TEAM=""
    build_publisher
    ARGS=(-priority "$PRIORITY" -team "$TEAM" -title "SRE end-to-end test incident")
    [[ -n "$CONTACT" ]] && ARGS+=(-contact-type "$CONTACT")
    OUT="$(publish "${ARGS[@]}")"
    echo "$OUT"
    INC="$(printf '%s' "$OUT" | sed -n 's/^published incident.created for \([^ ]*\) .*/\1/p')"
    echo "$INC" > "$STATE"
    echo "==> incident $INC published at $(TZ=Asia/Kolkata date '+%H:%M:%S IST'). Next: $0 watch"
    ;;
  ack)
    GESTURE="${1:?ack assign|status|comment [incident]}"; INC="${2:-$(last_incident)}"
    case "$GESTURE" in assign) EV=assigned ;; status) EV=acknowledged ;; comment) EV=comment ;; *) echo "ack assign|status|comment" >&2; exit 2 ;; esac
    build_publisher
    publish -event "$EV" -incident-id "$INC"
    echo "==> sent at $(TZ=Asia/Kolkata date '+%H:%M:%S IST'). Watch for 'ladder cancelled' (assign, status) or nothing (comment)."
    ;;
  watch)
    INC="${1:-$(last_incident)}"
    echo "==> following $INC (ctrl-c to stop)"
    docker compose logs -f --since 30m csm-notification-service 2>/dev/null \
      | grep --line-buffered "$INC" \
      | grep --line-buffered -E 'routing put|ladder scheduled|would notify|ALERT TRIGGERED|ladder cancelled|exhausted|half acknowledged|level cannot be called|scheduled no|configuration does not' \
      | sed -E -u 's/^csm-notification-service-1 *\| *//'
    ;;
  status)
    INC="${1:-$(last_incident)}"
    for ns in "incident:escalation:state:" "incident:escalation:sre:state:"; do
      printf '%-36s ' "$ns"
      docker compose exec -T redis redis-cli EXISTS "$ns$INC" | tr -d '\r' | sed 's/1/running/;s/0/no ladder (never started, finished or stopped)/'
    done
    ;;
  oncall) oncall ;;
  down)
    "${PSQL[@]}" -c "DELETE FROM team_schedule_assignment WHERE note = 'QA-SRE-E2E'" >/dev/null && echo "==> test rota removed"
    docker compose up -d csm-notification-service >/dev/null && echo "==> notification service back on the base stack"
    "${COMPOSE[@]}" stop entity-gateway-shim >/dev/null 2>&1 || true
    ;;
  *) sed -n '/^# Usage:/,/^set -euo/p' "$0" | sed 's/^# \{0,1\}//;$d'; exit 2 ;;
esac
