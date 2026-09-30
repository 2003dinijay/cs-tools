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
# Exercise the CRE call-escalation ladder end to end, contacting nobody.
#
# It runs the REAL engine -- the same code cmd/server runs -- against a real
# Redis, on the `log` channel. Event decoding, shift derivation, rule matching,
# the priority clock, the durable state, the wake loop, cancellation and the
# work note are all genuinely exercised. Nothing is dialled and nothing is
# posted to a chat space, so this needs no Twilio account and interrupts
# nobody.
#
# What it does NOT exercise: who each rung resolves to. Recipients come from a
# local roster whose names ARE the rung names, so you see which rung fires
# rather than which person. Resolving real people needs entity-service
# reachable and CUSTOMER_ENTITY_BASE_URL set.
#
# Usage:
#   ./scripts/csm-compose/test-cre-ladder.sh              # every scenario
#   ./scripts/csm-compose/test-cre-ladder.sh timings      # just one
#   KEEP_REDIS=1 ./scripts/csm-compose/test-cre-ladder.sh # leave Redis running
#
# Scenarios: timings | ack | half-ack | shifts | not-abt

set -euo pipefail

REDIS_NAME="${REDIS_NAME:-cre-ladder-redis}"
REDIS_PORT="${REDIS_PORT:-16379}"
REDIS_ADDR="127.0.0.1:${REDIS_PORT}"

# One ladder minute in wall-clock time. 150ms turns a 44-minute P1 ladder into
# about seven seconds; unset it to watch the real thing.
MINUTE="${MINUTE:-150ms}"
TICK="${TICK:-60ms}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
service_dir="${repo_root}/integrations/csm-notification-service"

run() {
  # --channel log is what makes this safe: the ladder runs in full and reaches
  # nobody. Every other flag is about which ladder to run.
  (cd "${service_dir}" && go run ./cmd/escalation-local \
      --channel log --redis "${REDIS_ADDR}" \
      --minute "${MINUTE}" --tick "${TICK}" "$@" 2>/dev/null)
}

heading() {
  printf '\n\033[1m%s\033[0m\n' "$1"
  printf '%s\n' "$(printf '%.0s-' $(seq 1 ${#1}))"
}

start_redis() {
  if docker ps --format '{{.Names}}' | grep -qx "${REDIS_NAME}"; then
    echo "redis: already running as ${REDIS_NAME}"
    return
  fi
  docker rm -f "${REDIS_NAME}" >/dev/null 2>&1 || true
  docker run -d --name "${REDIS_NAME}" -p "127.0.0.1:${REDIS_PORT}:6379" \
    redis:7-alpine >/dev/null
  # The ladder's whole state lives here, so fail early rather than half way
  # through a scenario.
  for _ in $(seq 1 20); do
    if [ "$(docker exec "${REDIS_NAME}" redis-cli ping 2>/dev/null)" = "PONG" ]; then
      echo "redis: up on ${REDIS_ADDR}"
      return
    fi
    sleep 0.5
  done
  echo "redis did not come up on ${REDIS_ADDR}" >&2
  exit 1
}

cleanup() {
  [ -n "${KEEP_REDIS:-}" ] && { echo; echo "redis left running as ${REDIS_NAME}"; return; }
  docker rm -f "${REDIS_NAME}" >/dev/null 2>&1 || true
  echo
  echo "redis removed"
}

scenario_timings() {
  heading "1. The clock, per priority"
  cat <<'NOTE'
The rung order is the thing to check:
  first responders -> the team's own lead -> three team leads -> CRE head -> CS head
If LEVEL_1 and LEVEL_2 look swapped, it is routing by the previous model.

Expected openings, from the specification:
  S0   0  +1  +4  +8  +12
  S1  +6  +9 +18 +28  +38
NOTE
  for p in P0 P1; do
    printf '\n  --- %s ---\n' "${p}"
    run --priority "${p}" --shift LK | sed -n '/the ladder the engine scheduled/,/^$/p'
  done
}

scenario_ack() {
  heading "2. Acknowledgement stops it"
  echo "Both gestures arrive, so the outstanding calls are cancelled."
  run --priority P0 --shift LK --cancel-after 1s | tail -6
}

scenario_half_ack() {
  heading "3. One gesture alone does NOT stop it"
  cat <<'NOTE'
Acknowledgement is a move out of NEW AND a public comment. A status change on
its own is what a dispatcher does while triaging a queue, so the ladder must
keep climbing. Watch for "half acknowledged" in the log and calls continuing.
NOTE
  local out
  out="$( (cd "${service_dir}" && go run ./cmd/escalation-local \
      --channel log --redis "${REDIS_ADDR}" --minute "${MINUTE}" --tick "${TICK}" \
      --priority P0 --shift LK --cancel-after 1s --cancel-by status 2>&1) )"

  printf '\n  the engine says:\n'
  echo "${out}" | grep -i 'half acknowledged' | sed 's/^/    /' || true

  printf '\n  and the rungs after it:\n'
  echo "${out}" | grep -oE 'LEVEL_[0-4]' | sort -u | tr '\n' ' ' | sed 's/^/    /'
  printf '\n'

  cat <<'NOTE'

  PASS when "half acknowledged ... stillNeeds a public comment" appears and
  rungs above the one it had reached still fire.

  Ignore the "Acknowledged : N call(s) cancelled" line at the very end -- that
  is this harness retiring its own ladder on exit so a later run does not
  resume it, not the engine accepting one gesture.
NOTE
}

scenario_shifts() {
  heading "4. Each shift routes by its own rule"
  echo "LK_MORNING -> R1a, LK_EVENING -> R4a, USA -> R5, USA_WEEKEND -> R6."
  for s in LK_MORNING LK_EVENING USA USA_WEEKEND; do
    printf '\n  --- %s ---\n' "${s}"
    run --priority P1 --shift "${s}" | sed -n '/the ladder the engine scheduled/,/^$/p' | head -8
  done
}

scenario_not_abt() {
  heading "5. An incident on no ABT team"
  echo "Routes R3 rather than R2: LEVEL_0 becomes one nominee from each ABT team."
  run --priority P1 --shift LK --not-abt | sed -n '/the ladder the engine scheduled/,/^$/p' | head -8
}

main() {
  command -v docker >/dev/null || { echo "docker is required" >&2; exit 1; }
  command -v go >/dev/null || { echo "go is required" >&2; exit 1; }

  start_redis
  trap cleanup EXIT

  case "${1:-all}" in
    timings)  scenario_timings ;;
    ack)      scenario_ack ;;
    half-ack) scenario_half_ack ;;
    shifts)   scenario_shifts ;;
    not-abt)  scenario_not_abt ;;
    all)
      scenario_timings
      scenario_ack
      scenario_half_ack
      scenario_shifts
      scenario_not_abt
      ;;
    *)
      echo "unknown scenario: $1" >&2
      echo "use one of: timings ack half-ack shifts not-abt all" >&2
      exit 2
      ;;
  esac
}

main "$@"
