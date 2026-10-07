-- Prerequisite for til-backend. Run this once against the target database
-- before starting the app -- db.py only ever verifies this table exists
-- (SHOW TABLES LIKE) and fails loudly if it doesn't; it never creates
-- anything itself, so an operator always knows exactly what ran against
-- a real database. Same posture as the PAR Legacy Migration Tool's own
-- sql/create_migration_tables.sql.
--
-- Usage:
--   mysql -h <host> -P <port> -u <user> -p <database> < sql/create_til_tables.sql

CREATE TABLE IF NOT EXISTS til_submissions (
    id                 VARCHAR(36)   NOT NULL PRIMARY KEY,
    -- Added after the table already had rows in some deployments -- NOT
    -- NULL DEFAULT '' rather than nullable, so an existing row reads as "no
    -- title" (empty, falsy) consistently with how a title-less entry is
    -- meant to display, without needing a separate NULL-check anywhere.
    -- Every NEW submission is required to provide one (validation.py);
    -- this default only ever applies to rows that predate this column.
    title              VARCHAR(150)  NOT NULL DEFAULT '',
    who                VARCHAR(200)  NOT NULL,
    where_             VARCHAR(20)   NOT NULL,
    where_detail       VARCHAR(200)  NULL,
    what               TEXT          NOT NULL,
    submitted_by_email VARCHAR(255)  NOT NULL,
    -- ISO 8601 UTC string (e.g. "2026-10-04T09:28:36.606775+00:00"), not a
    -- native DATETIME -- keeps this column a drop-in match for the values
    -- db.py already produces/compares as plain strings (keyset pagination
    -- does `WHERE created_at < %s` on this same string form).
    created_at         VARCHAR(40)   NOT NULL,
    INDEX idx_til_submissions_created_at (created_at DESC),
    -- Backs the "Submitted by (email)" search scope and the "My entries"
    -- tab filter -- both now run as a real WHERE clause in list_submissions,
    -- not a client-side scan of whatever page happened to be fetched.
    INDEX idx_til_submissions_submitted_by_email (submitted_by_email),
    -- Backs filtering by Customer/Partner/Internal/Other, same reasoning.
    INDEX idx_til_submissions_where (where_),
    -- Backs the default "What was learned" search. FULLTEXT rather than a
    -- regular index -- a plain B-tree index can't accelerate a `LIKE
    -- '%word%'` scan at all (the leading wildcard makes it unusable), and
    -- `what` is the one column actually worth searching at real scale.
    -- MySQL's own tokenizer splits on non-alphanumeric characters, so HTML
    -- tags in the stored markup naturally act as word boundaries rather
    -- than polluting the index.
    FULLTEXT INDEX idx_til_submissions_what_fulltext (what)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
