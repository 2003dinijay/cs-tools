-- Sweeps expired rows out of sftpgo_auth_sessions in small batches, so the
-- delete never holds a single long-running lock over the whole table.
--
-- Sessions are otherwise only removed lazily, when GetSession happens to read
-- back the exact expired request_id (see deleteExpiredSession in
-- internal/service/database.go). A keyboard-interactive login the user
-- abandons half-way leaves its row behind forever without this.
--
-- This is a PROCEDURE, not a function: only a procedure invoked via CALL can
-- COMMIT between iterations of the loop, which is what lets each batch
-- release its lock before the next one starts.
CREATE OR REPLACE PROCEDURE sftpgo_auth_cleanup_expired_sessions(batch_size INT DEFAULT 1000)
LANGUAGE plpgsql
AS $$
DECLARE
    deleted_count INT;
BEGIN
    LOOP
        DELETE FROM sftpgo_auth_sessions
        WHERE ctid IN (
            SELECT ctid FROM sftpgo_auth_sessions
            WHERE expires_at < now()
            LIMIT batch_size
        );
        GET DIAGNOSTICS deleted_count = ROW_COUNT;
        COMMIT;

        EXIT WHEN deleted_count < batch_size;

        -- Brief pause between batches so a large backlog doesn't hammer the
        -- table back-to-back; keeps this gentle on concurrent readers/writers.
        PERFORM pg_sleep(0.1);
    END LOOP;
END;
$$;

-- Schedule the procedure to run inside the database itself (not via a Go-side
-- ticker), so cleanup keeps running on its own schedule regardless of service
-- restarts and works the same way across multiple service instances.
--
-- pg_cron requires superuser to CREATE EXTENSION and isn't available on every
-- managed Postgres instance (some require it pre-enabled at the instance
-- level, not installable from a normal migration). So this migration never
-- attempts CREATE EXTENSION pg_cron itself -- doing so would fail the
-- migration on any instance without that access, breaking every deployment
-- that doesn't already have it.
--
-- If pg_cron is already installed (checked via pg_extension, not created
-- here), self-schedule the cleanup once daily. If it isn't installed, this
-- block is a no-op: the procedure above still exists and must be invoked
-- externally -- see README.md's "Session Cleanup" section for the
-- operational requirement in that case.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
        -- Runs daily at 08:00 IST (UTC+5:30) = 02:30 UTC. pg_cron schedules
        -- run in the server's configured timezone (UTC by default) unless
        -- cron.timezone is set -- if this instance's cron.timezone is ever
        -- changed, this expression must be re-derived for the new timezone.
        PERFORM cron.schedule(
            'sftpgo-auth-session-cleanup',
            '30 2 * * *',
            'CALL sftpgo_auth_cleanup_expired_sessions()'
        );
    END IF;
END $$;
