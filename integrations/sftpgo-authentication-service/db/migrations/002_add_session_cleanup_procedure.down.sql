-- Reverts 002_add_session_cleanup_procedure.up.sql

-- Unschedule the cron job first, if pg_cron is installed and the job exists.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
        PERFORM cron.unschedule(jobid)
        FROM cron.job
        WHERE jobname = 'sftpgo-auth-session-cleanup';
    END IF;
END $$;

DROP PROCEDURE IF EXISTS sftpgo_auth_cleanup_expired_sessions(INT);
