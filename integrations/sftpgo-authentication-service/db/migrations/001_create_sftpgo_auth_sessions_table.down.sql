-- Reverts 001_create_sftpgo_auth_sessions_table.up.sql
DROP TRIGGER IF EXISTS trg_sftpgo_auth_sessions_updated_at ON sftpgo_auth_sessions;
DROP FUNCTION IF EXISTS set_sftpgo_auth_sessions_updated_at();
DROP TABLE IF EXISTS sftpgo_auth_sessions;
