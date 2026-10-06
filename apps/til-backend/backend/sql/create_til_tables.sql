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
    INDEX idx_til_submissions_created_at (created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
