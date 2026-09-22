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

-- Tell the issue what record it became.
--
-- Someone files an issue, we create SR-GH-000004 from it, and nothing on the
-- issue ever says so -- the person who filed it has no way to know the request
-- was picked up, or what to quote when asking about it. Every outbound event
-- until now described a CHANGE to a record that already existed (closed,
-- assigned, plan dates, comments), so creation had nothing to ride on.
--
-- Fires on INSERT, and only when the record already carries an issue number:
-- a service request raised in the portal has no issue to comment on.
CREATE OR REPLACE FUNCTION trg_github_record_created()
RETURNS TRIGGER AS $$
DECLARE
    gh RECORD;
    wi RECORD;
    catalog_name TEXT;
    priority_val TEXT;
    environment  TEXT;
    field_count  INT := 0;
BEGIN
    SELECT agr.owner, agr.repository, linked_wi.github_issue_number
      INTO gh
      FROM work_item linked_wi
      JOIN account_github_repo agr ON agr.account_id = linked_wi.account_id
     WHERE linked_wi.id = NEW.id
       AND linked_wi.github_issue_number IS NOT NULL
       AND agr.is_active;

    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT work_item.number, work_item.type INTO wi
      FROM work_item WHERE work_item.id = NEW.id;

    -- The announcement repeats the fields the template asked for, so the
    -- reporter can confirm what was captured without opening the portal.
    -- These live in service_request.json_data under the keys the extractor
    -- writes; a change request has no equivalent and simply shows nothing.
    IF TG_TABLE_NAME = 'service_request' THEN
        catalog_name := NEW.category::TEXT;
        priority_val := NEW.json_data ->> 'u_priority';
        environment  := NEW.json_data ->> 'u_environment_details';
        field_count  := (SELECT COUNT(*) FROM jsonb_object_keys(COALESCE(NEW.json_data, '{}'::jsonb)));
    END IF;

    -- Field names match what the case-update workflow already parses
    -- (case_number, case_sys_id), so acting on this needs one new branch in
    -- that workflow rather than a whole new file in every customer repository.
    -- record_type is the one addition, so the comment can say which kind of
    -- record was raised, plus the captured fields the announcement
    -- repeats back. Nine properties; GitHub rejects more than ten.
    INSERT INTO github_outbound_queue (event, work_item_id, owner, repository, issue_number, payload)
    VALUES ('record_created', NEW.id, gh.owner, gh.repository, gh.github_issue_number,
            jsonb_build_object(
                'action',              'created',
                'github_issue_number', gh.github_issue_number,
                'case_number',         wi.number,
                'case_sys_id',         NEW.id,
                'record_type',         wi.type,
                'catalog',             catalog_name,
                'priority',            priority_val,
                'environment',         environment,
                'field_count',         field_count));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER service_request_created_github_notice
    AFTER INSERT ON service_request
    FOR EACH ROW EXECUTE FUNCTION trg_github_record_created();

CREATE TRIGGER change_request_created_github_notice
    AFTER INSERT ON change_request
    FOR EACH ROW EXECUTE FUNCTION trg_github_record_created();
