-- Migration to create the sftpgo_auth_sessions table for the auth hook
CREATE TABLE IF NOT EXISTS sftpgo_auth_sessions (
  request_id VARCHAR(255) NOT NULL,
  session_data JSONB NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (request_id)
);

CREATE INDEX IF NOT EXISTS idx_sftpgo_auth_sessions_expires_at ON sftpgo_auth_sessions (expires_at);

-- Postgres has no native ON UPDATE CURRENT_TIMESTAMP, so touch updated_at via trigger.
CREATE OR REPLACE FUNCTION set_sftpgo_auth_sessions_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_sftpgo_auth_sessions_updated_at ON sftpgo_auth_sessions;
CREATE TRIGGER trg_sftpgo_auth_sessions_updated_at
  BEFORE UPDATE ON sftpgo_auth_sessions
  FOR EACH ROW
  EXECUTE FUNCTION set_sftpgo_auth_sessions_updated_at();
