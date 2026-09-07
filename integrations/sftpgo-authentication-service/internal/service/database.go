// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/config"

	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/log"
	"github.com/wso2-open-operations/cs-tools/integrations/sftpgo-authentication-service/internal/models"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver (database/sql compatible)
)

// ErrSessionNotFound is returned when a requested session is not found or has expired.
var ErrSessionNotFound = errors.New("session not found or expired")

// DBService handles all database interactions.
type DBService struct {
	// db is the underlying SQL database connection.
	db *sql.DB
	// logger is the application-wide logger.
	logger *log.AppLogger
}

// NewDBService creates a new DBService and establishes a database connection.
func NewDBService(cfg *config.Config, logger *log.AppLogger) (*DBService, error) {
	db, err := sql.Open("pgx", cfg.DBConnString)
	if err != nil {
		return nil, logger.Errorf("failed to open database connection: %v", err)
	}

	if err = db.Ping(); err != nil {
		return nil, logger.Errorf("failed to ping database: %v", err)
	}

	// Connection Pooling Configuration
	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)

	logger.Info("Successfully connected to the PostgreSQL database.")
	return &DBService{db: db, logger: logger}, nil
}

// SaveSession saves session data to the database.
func (s *DBService) SaveSession(requestID string, data models.SessionData) error {
	if s.db == nil {
		s.logger.Warn("Database not configured. Cannot save session %s.", requestID)
		return nil // Not a fatal error
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return s.logger.Errorf("failed to marshal session data: %v", err)
	}

	expiresAt := time.Now().Add(15 * time.Minute)
	query := `INSERT INTO sftpgo_auth_sessions (request_id, session_data, expires_at)
              VALUES ($1, $2, $3)
              ON CONFLICT (request_id) DO UPDATE SET session_data = EXCLUDED.session_data, expires_at = EXCLUDED.expires_at, updated_at = now()`

	_, err = s.db.Exec(query, requestID, jsonData, expiresAt)
	if err != nil {
		return s.logger.Errorf("failed to save session to database: %v", err)
	}
	s.logger.Debug("Session %s saved successfully to the database.", requestID)
	return nil
}

// GetSession retrieves session data from the database.
func (s *DBService) GetSession(requestID string) (models.SessionData, error) {
	if s.db == nil {
		s.logger.Warn("Database not configured. Cannot get session %s.", requestID)
		return models.SessionData{}, ErrSessionNotFound
	}

	var jsonData []byte
	var expiresAt time.Time
	data := models.SessionData{}

	query := `SELECT session_data, expires_at FROM sftpgo_auth_sessions WHERE request_id = $1`
	row := s.db.QueryRow(query, requestID)

	if err := row.Scan(&jsonData, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return data, ErrSessionNotFound
		}
		return data, s.logger.Errorf("failed to retrieve session data: %v", err)
	}

	if time.Now().After(expiresAt) {
		// Delete only the row we just read as expired, conditioned on
		// expires_at still matching that value. Without this, a concurrent
		// SaveSession could upsert a fresh, non-expired row for the same
		// request_id between our read above and this delete; an unconditional
		// `DELETE ... WHERE request_id = $1` would remove that fresh row
		// instead of the stale one we actually observed.
		_ = s.deleteExpiredSession(requestID, expiresAt)
		return data, ErrSessionNotFound
	}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return data, s.logger.Errorf("failed to unmarshal session data: %v", err)
	}
	s.logger.Debug("Session %s retrieved successfully.", requestID)
	return data, nil
}

// DeleteSession removes a session from the database. Used when a caller
// already holds the session's identity (the auth flow completed or failed)
// and unconditionally wants that request_id's row gone.
func (s *DBService) DeleteSession(requestID string) error {
	if s.db == nil {
		return nil
	}
	query := `DELETE FROM sftpgo_auth_sessions WHERE request_id = $1`
	_, err := s.db.Exec(query, requestID)
	if err != nil {
		return s.logger.Errorf("failed to delete session: %v", err)
	}
	s.logger.Debug("Session %s deleted successfully.", requestID)
	return nil
}

// deleteExpiredSession removes a session row only if its expires_at still
// matches expiresAt, the value GetSession just read as expired. This closes a
// race where a concurrent SaveSession upserts a fresh row for the same
// request_id between GetSession's read and this delete: an unconditional
// delete by request_id alone could remove that fresh row instead of the
// stale one actually observed.
func (s *DBService) deleteExpiredSession(requestID string, expiresAt time.Time) error {
	if s.db == nil {
		return nil
	}
	query := `DELETE FROM sftpgo_auth_sessions WHERE request_id = $1 AND expires_at = $2`
	_, err := s.db.Exec(query, requestID, expiresAt)
	if err != nil {
		return s.logger.Errorf("failed to delete expired session: %v", err)
	}
	s.logger.Debug("Expired session %s deleted successfully.", requestID)
	return nil
}
