#!/bin/sh
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
# One-shot init: applies entity-service's and sre-alert-ingestion-service's
# raw SQL migrations (neither service wires up a migration tool -- see
# apps/csm-portal/README.md), then loads dummy seed data into entity-service's
# database. Runs as the "migrate" compose service, which every dependent
# service waits on via `depends_on: condition: service_completed_successfully`.
set -eu

export PGPASSWORD="${POSTGRES_PASSWORD}"
PSQL="psql -h ${POSTGRES_HOST} -p ${POSTGRES_PORT} -U ${POSTGRES_USER} -v ON_ERROR_STOP=1"

echo "[migrate] waiting for postgres..."
until $PSQL -d postgres -c 'select 1' > /dev/null 2>&1; do
  sleep 1
done

echo "[migrate] ensuring databases exist"
$PSQL -d postgres -tc "SELECT 1 FROM pg_database WHERE datname = '${ENTITY_DB_NAME}'" | grep -q 1 || \
  $PSQL -d postgres -c "CREATE DATABASE \"${ENTITY_DB_NAME}\""
$PSQL -d postgres -tc "SELECT 1 FROM pg_database WHERE datname = '${SRE_ALERT_DB_NAME}'" | grep -q 1 || \
  $PSQL -d postgres -c "CREATE DATABASE \"${SRE_ALERT_DB_NAME}\""

# entity-service ships no migration tool and its raw .up.sql files are not
# all safely re-runnable (most guard with IF NOT EXISTS, but at least one
# ADD CONSTRAINT does not -- a real gap in those files, not something to
# patch here). So this script applies them at most once per database
# lifetime, gated on whether the schema already looks migrated, rather than
# re-running the full set on every `docker compose up`.
if $PSQL -d "${ENTITY_DB_NAME}" -tc "SELECT 1 FROM information_schema.tables WHERE table_name = 'work_item'" | grep -q 1; then
  echo "[migrate] entity-service schema already present, skipping migrations"
else
  echo "[migrate] applying entity-service migrations"
  for f in $(ls /migrations/entity-service/*.up.sql | sort); do
    echo "[migrate]   $f"
    $PSQL -d "${ENTITY_DB_NAME}" -f "$f"
  done
fi

if $PSQL -d "${SRE_ALERT_DB_NAME}" -tc "SELECT 1 FROM information_schema.tables WHERE table_name = 'alert_buffer'" | grep -q 1; then
  echo "[migrate] sre-alert-ingestion-service schema already present, skipping migrations"
else
  echo "[migrate] applying sre-alert-ingestion-service migrations"
  for f in $(ls /migrations/sre-alert-ingestion-service/*.up.sql | sort); do
    echo "[migrate]   $f"
    $PSQL -d "${SRE_ALERT_DB_NAME}" -f "$f"
  done
fi

echo "[migrate] loading entity-service seed data"
$PSQL -d "${ENTITY_DB_NAME}" -f /migrations/seed-entity-service.sql

echo "[migrate] done"
