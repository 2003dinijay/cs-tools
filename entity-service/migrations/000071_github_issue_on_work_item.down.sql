-- Put the GitHub issue link back on "case".
--
-- LOSSY BY NATURE, and deliberately not disguised: work_item rows that are not
-- cases cannot be represented on "case" at all, so a service request or
-- incident linked to an issue loses that link on the way down. There is
-- nowhere for it to go.

BEGIN;

ALTER TABLE "case" ADD COLUMN IF NOT EXISTS github_issue_number INTEGER;

UPDATE "case" c
SET github_issue_number = wi.github_issue_number
FROM work_item wi
WHERE wi.id = c.id AND wi.github_issue_number IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_case_github_issue_number
    ON "case" (github_issue_number) WHERE github_issue_number IS NOT NULL;

DROP INDEX IF EXISTS idx_work_item_github_issue_number;
ALTER TABLE work_item DROP COLUMN IF EXISTS github_issue_number;

COMMIT;
