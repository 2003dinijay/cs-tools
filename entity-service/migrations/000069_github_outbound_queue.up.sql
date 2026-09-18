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

-- Work waiting to be pushed to GitHub: the outbound half of the change-request
-- sync, replacing ServiceNow's [GitHub Integration] flows.
--
-- A SEPARATE QUEUE FROM THE NOTIFICATION OUTBOX, ON PURPOSE. That one claims a
-- row at read time and never retries, because a lost email beats a duplicate
-- one. This calls someone else's service, where the trade-off inverts: GitHub
-- returns 502s and rate limits that succeed on the next attempt, so a row here
-- is retried with backoff and only abandoned after a bounded number of tries.
--
-- THE GATE IS git_reference. A change request with no linked issue has nowhere
-- to push, so nothing is enqueued for it -- the same condition ServiceNow
-- expressed as u_git_referenceISNOTEMPTY on every one of these flows.
CREATE TABLE IF NOT EXISTS github_outbound_queue (
    id BIGSERIAL PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- What happened, which decides what gets pushed.
    event VARCHAR(40) NOT NULL,
    -- The change request this is about. Everything we push is a comment, label
    -- or state change on its linked issue.
    change_request_id UUID NOT NULL REFERENCES change_request(id) ON DELETE CASCADE,
    -- The issue URL captured AT ENQUEUE TIME rather than looked up on delivery.
    -- If someone re-points git_reference afterwards, this row still belongs to
    -- the issue the change was actually about.
    git_reference TEXT NOT NULL,
    -- Event-specific detail: the comment body, the columns that changed.
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Retry state.
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Bounded, and never the raw upstream body: an error from GitHub can carry
    -- content we are not allowed to store or log.
    last_error VARCHAR(500),
    delivered_on TIMESTAMPTZ,

    CONSTRAINT chk_github_outbound_status
        CHECK (status IN ('PENDING', 'DELIVERED', 'FAILED'))
);

-- The worker's only query: what is due now. Partial, so the index stays the
-- size of the backlog rather than the size of the history.
CREATE INDEX IF NOT EXISTS idx_github_outbound_due
    ON github_outbound_queue (next_attempt_on)
    WHERE status = 'PENDING';

CREATE INDEX IF NOT EXISTS idx_github_outbound_cr
    ON github_outbound_queue (change_request_id);

-- Enqueue a change-request event, but only when there is an issue to push to.
CREATE OR REPLACE FUNCTION trg_github_outbound_cr()
RETURNS TRIGGER AS $$
DECLARE
    ref TEXT := NEW.git_reference;
    ev  TEXT;
    diff JSONB := '{}'::jsonb;
    col TEXT;
    oldv JSONB;
    newv JSONB := to_jsonb(NEW);
BEGIN
    IF ref IS NULL OR ref = '' THEN
        RETURN NULL;
    END IF;

    IF TG_OP = 'INSERT' THEN
        ev := 'cr_created';
    ELSE
        ev := 'cr_updated';
        oldv := to_jsonb(OLD);
        FOR col IN SELECT jsonb_object_keys(newv) LOOP
            IF col <> 'updated_on' AND oldv -> col IS DISTINCT FROM newv -> col THEN
                diff := diff || jsonb_build_object(col,
                    jsonb_build_object('from', oldv -> col, 'to', newv -> col));
            END IF;
        END LOOP;
        -- A sync pass that rewrote the row with identical values is not news
        -- worth putting on someone's issue.
        IF diff = '{}'::jsonb THEN
            RETURN NULL;
        END IF;
    END IF;

    INSERT INTO github_outbound_queue (event, change_request_id, git_reference, payload)
    VALUES (ev, NEW.id, ref, jsonb_build_object('changes', diff));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS change_request_github_outbound ON change_request;
CREATE TRIGGER change_request_github_outbound
    AFTER INSERT OR UPDATE ON change_request
    FOR EACH ROW EXECUTE FUNCTION trg_github_outbound_cr();

-- Enqueue a comment, resolving the change request it belongs to. A comment on
-- a work item that is not a change request, or on one with no linked issue,
-- enqueues nothing.
CREATE OR REPLACE FUNCTION trg_github_outbound_comment()
RETURNS TRIGGER AS $$
DECLARE
    ref TEXT;
BEGIN
    SELECT cr.git_reference INTO ref
    FROM change_request cr
    WHERE cr.id = NEW.work_item_id
      AND COALESCE(cr.git_reference, '') <> '';

    IF ref IS NULL THEN
        RETURN NULL;
    END IF;

    INSERT INTO github_outbound_queue (event, change_request_id, git_reference, payload)
    VALUES ('comment_added', NEW.work_item_id, ref,
            jsonb_build_object('commentId', NEW.id,
                               'content', NEW.content,
                               'createdBy', NEW.created_by,
                               'type', NEW.type));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS comment_github_outbound ON comment;
CREATE TRIGGER comment_github_outbound
    AFTER INSERT ON comment
    FOR EACH ROW EXECUTE FUNCTION trg_github_outbound_comment();
