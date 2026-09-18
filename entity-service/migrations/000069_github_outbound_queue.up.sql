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
-- THE GATE IS THE PARENT CASE'S ISSUE NUMBER, matching ServiceNow's own flow:
-- "Change Request Created where Parent is not empty", then look up the case by
-- that parent and read the issue number off it. A change request has no issue
-- of its own -- it reaches GitHub only through the case it belongs to, and one
-- with no parent, or whose parent is not linked, is not our business.
CREATE TABLE IF NOT EXISTS github_outbound_queue (
    id BIGSERIAL PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- What happened, which decides what gets pushed.
    event VARCHAR(40) NOT NULL,
    -- The change request this is about. Everything we push is a comment, label
    -- or state change on its linked issue.
    change_request_id UUID NOT NULL REFERENCES change_request(id) ON DELETE CASCADE,
    -- Resolved AT ENQUEUE TIME rather than on delivery: if the case is
    -- re-linked afterwards, this row still belongs to the issue the change was
    -- actually about.
    owner VARCHAR(100) NOT NULL,
    repository VARCHAR(200) NOT NULL,
    issue_number INTEGER NOT NULL,
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
    gh RECORD;
    ev  TEXT;
    diff JSONB := '{}'::jsonb;
    col TEXT;
    oldv JSONB;
    newv JSONB := to_jsonb(NEW);
BEGIN
    -- Parent case -> its issue number -> the account's repository. All three
    -- must be present; any one missing means there is nowhere to push.
    SELECT agr.owner, agr.repository, c.github_issue_number
      INTO gh
      FROM work_item cr_wi
      JOIN work_item case_wi ON case_wi.id = cr_wi.parent_id
      JOIN "case" c          ON c.id = case_wi.id
      JOIN account_github_repo agr ON agr.account_id = case_wi.account_id
     WHERE cr_wi.id = NEW.id
       AND c.github_issue_number IS NOT NULL
       AND agr.is_active;

    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    IF TG_OP = 'INSERT' THEN
        ev := 'cr_created';
    ELSE
        ev := 'cr_updated';
        oldv := to_jsonb(OLD);
        -- Only the four columns ServiceNow's trigger watched: state, assigned
        -- to, planned start, planned end. Diffing every column would enqueue
        -- pushes for changes the integration being replaced never sent, onto
        -- an issue a customer can read.
        FOR col IN SELECT unnest(ARRAY['state','planned_start_date','planned_end_date']) LOOP
            IF oldv -> col IS DISTINCT FROM newv -> col THEN
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

    INSERT INTO github_outbound_queue (event, change_request_id, owner, repository, issue_number, payload)
    VALUES (ev, NEW.id, gh.owner, gh.repository, gh.github_issue_number,
            jsonb_build_object('changes', diff));
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
    gh RECORD;
BEGIN
    -- Same walk as above: a comment on a change request reaches GitHub only
    -- through the case that change request belongs to.
    SELECT agr.owner, agr.repository, c.github_issue_number
      INTO gh
      FROM work_item cr_wi
      JOIN work_item case_wi ON case_wi.id = cr_wi.parent_id
      JOIN "case" c          ON c.id = case_wi.id
      JOIN account_github_repo agr ON agr.account_id = case_wi.account_id
     WHERE cr_wi.id = NEW.work_item_id
       AND c.github_issue_number IS NOT NULL
       AND agr.is_active;

    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    INSERT INTO github_outbound_queue (event, change_request_id, owner, repository, issue_number, payload)
    VALUES ('comment_added', NEW.work_item_id, gh.owner, gh.repository, gh.github_issue_number,
            jsonb_build_object('commentId', NEW.id,
                               'content', NEW.content,
                               'createdBy', NEW.created_by,
                               'type', NEW.type));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Assignment is the fourth field ServiceNow watched, and it lives on
-- work_item rather than change_request -- so it needs its own trigger on that
-- table, restricted to rows that are change requests.
CREATE OR REPLACE FUNCTION trg_github_outbound_assignment()
RETURNS TRIGGER AS $$
DECLARE
    gh RECORD;
BEGIN
    IF NEW.type <> 'CHANGE_REQUEST' OR OLD.assigned_to_id IS NOT DISTINCT FROM NEW.assigned_to_id THEN
        RETURN NULL;
    END IF;

    SELECT agr.owner, agr.repository, c.github_issue_number
      INTO gh
      FROM work_item case_wi
      JOIN "case" c ON c.id = case_wi.id
      JOIN account_github_repo agr ON agr.account_id = case_wi.account_id
     WHERE case_wi.id = NEW.parent_id
       AND c.github_issue_number IS NOT NULL
       AND agr.is_active;

    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    INSERT INTO github_outbound_queue (event, change_request_id, owner, repository, issue_number, payload)
    VALUES ('cr_updated', NEW.id, gh.owner, gh.repository, gh.github_issue_number,
            jsonb_build_object('changes', jsonb_build_object('assigned_to_id',
                jsonb_build_object('from', OLD.assigned_to_id, 'to', NEW.assigned_to_id))));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS work_item_assignment_github_outbound ON work_item;
CREATE TRIGGER work_item_assignment_github_outbound
    AFTER UPDATE OF assigned_to_id ON work_item
    FOR EACH ROW EXECUTE FUNCTION trg_github_outbound_assignment();

DROP TRIGGER IF EXISTS comment_github_outbound ON comment;
CREATE TRIGGER comment_github_outbound
    AFTER INSERT ON comment
    FOR EACH ROW EXECUTE FUNCTION trg_github_outbound_comment();
