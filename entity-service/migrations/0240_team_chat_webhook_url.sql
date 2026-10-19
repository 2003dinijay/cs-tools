-- Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License.
-- You may obtain a copy of the License at
--
-- http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied.  See the License for the
-- specific language governing permissions and limitations
-- under the License.

-- team.chat_webhook_url: a team's own Google Chat incoming-webhook URL, the
-- replacement for configuring it by hand in csm-notification-service's
-- GOOGLE_CHAT_SPACES env var. Nullable -- most teams have none yet, and a
-- team with no URL here simply has nowhere specific to route a Chat alert,
-- same as today. Like team.key/group_email (0117/0152), this is never
-- written by the ServiceNow sync (0033): it is platform-owned configuration,
-- not synced vocabulary, so a plain nullable column needs no trigger or
-- backfill to coexist safely with the sync's own INSERT/UPSERT.
ALTER TABLE team ADD COLUMN IF NOT EXISTS chat_webhook_url TEXT;

COMMENT ON COLUMN team.chat_webhook_url IS
    'The team''s own Google Chat incoming-webhook URL, set directly by platform operators (never by the ServiceNow sync). NULL means the team has no Chat space of its own.';
