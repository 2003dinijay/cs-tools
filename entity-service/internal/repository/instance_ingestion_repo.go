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

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// InstanceIngestionRepository defines the persistence operations for telemetry ingestion.
type InstanceIngestionRepository interface {
	ResolveProductVersionID(ctx context.Context, productName, version string) (*string, error)
	UpsertDeploymentNode(ctx context.Context, req domain.InstanceMetaRequest) (string, error)
	InsertDeploymentInformation(ctx context.Context, req domain.InstanceInfoRequest) (string, error)
	InsertUsageStats(ctx context.Context, req domain.InstanceUsageIngestRequest) error
	ImportDailyUsagesBatch(ctx context.Context, req domain.DailyUsageBatchRequest) (int, error)
	GetProjectSecretKey(ctx context.Context, projectKey string) (string, error)
}

type instanceIngestionRepo struct {
	db *Scoped
}

// NewInstanceIngestionRepository constructs an InstanceIngestionRepository backed by *Scoped.
func NewInstanceIngestionRepository(db *Scoped) InstanceIngestionRepository {
	return &instanceIngestionRepo{db: db}
}

func ensureCallerIdentity(ctx context.Context) context.Context {
	if _, ok := CallerIdentityFromContext(ctx); !ok {
		return WithSystemIdentity(ctx)
	}
	return ctx
}

// ResolveProductVersionID maps a product name/code and version string to a product_version.id UUID.
func (r *instanceIngestionRepo) ResolveProductVersionID(ctx context.Context, productName, version string) (*string, error) {
	ctx = ensureCallerIdentity(ctx)
	trimmedProduct := strings.TrimSpace(productName)
	trimmedVersion := strings.TrimSpace(version)
	if trimmedProduct == "" || trimmedVersion == "" {
		return nil, nil
	}

	const query = `
		SELECT pv.id::text
		FROM product_version pv
		JOIN product p ON p.id = pv.product_id
		WHERE (LOWER(p.name) = LOWER($1) OR LOWER(p.code) = LOWER($1) OR LOWER(p.name) LIKE '%' || LOWER($1) || '%')
		  AND pv.version = $2
		LIMIT 1`

	var id string
	err := r.db.QueryRow(ctx, query, trimmedProduct, trimmedVersion).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("resolve product version: %w", err)
	}
	return &id, nil
}

// UpsertDeploymentNode registers or updates a node in deployment_node.
func (r *instanceIngestionRepo) UpsertDeploymentNode(ctx context.Context, req domain.InstanceMetaRequest) (string, error) {
	ctx = ensureCallerIdentity(ctx)
	if strings.TrimSpace(req.NodeID) == "" {
		return "", &apierror.ValidationError{Msg: "nodeId is required"}
	}

	versionID, err := r.ResolveProductVersionID(ctx, req.Product, req.Version)
	if err != nil {
		return "", err
	}

	// Check if node exists by node_id.
	var existingID string
	err = r.db.QueryRow(ctx, "SELECT id::text FROM deployment_node WHERE node_id = $1 LIMIT 1", req.NodeID).Scan(&existingID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("check existing deployment node: %w", err)
	}

	if existingID != "" {
		const updateSQL = `
			UPDATE deployment_node
			SET project_key = $2, deployment_number = $3, product_version_id = $4, updated_on = NOW()
			WHERE id = $1::uuid`
		if _, err := r.db.Exec(ctx, updateSQL, existingID, req.ProjectKey, req.DeploymentNumber, versionID); err != nil {
			return "", fmt.Errorf("update deployment node: %w", err)
		}
		return existingID, nil
	}

	const insertSQL = `
		INSERT INTO deployment_node (node_id, project_key, deployment_number, product_version_id, created_on, updated_on)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id::text`
	var insertedID string
	if err := r.db.QueryRow(ctx, insertSQL, req.NodeID, req.ProjectKey, req.DeploymentNumber, versionID).Scan(&insertedID); err != nil {
		return "", fmt.Errorf("insert deployment node: %w", err)
	}
	return insertedID, nil
}

// InsertDeploymentInformation records host and JDK metadata snapshot.
func (r *instanceIngestionRepo) InsertDeploymentInformation(ctx context.Context, req domain.InstanceInfoRequest) (string, error) {
	ctx = ensureCallerIdentity(ctx)
	if strings.TrimSpace(req.NodeID) == "" {
		return "", &apierror.ValidationError{Msg: "nodeId is required"}
	}

	var nodeUUID string
	err := r.db.QueryRow(ctx, "SELECT id::text FROM deployment_node WHERE node_id = $1 LIMIT 1", req.NodeID).Scan(&nodeUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Create a default node record if missing so snapshot does not fail
			inserted, insErr := r.UpsertDeploymentNode(ctx, domain.InstanceMetaRequest{NodeID: req.NodeID})
			if insErr != nil {
				return "", fmt.Errorf("create missing deployment node for info: %w", insErr)
			}
			nodeUUID = inserted
		} else {
			return "", fmt.Errorf("find deployment node: %w", err)
		}
	}

	var deploymentInfoJSON []byte
	if req.DeploymentInfo != nil {
		deploymentInfoJSON, _ = json.Marshal(req.DeploymentInfo)
	}
	if len(deploymentInfoJSON) == 0 {
		deploymentInfoJSON = []byte("{}")
	}

	createdTime := req.CreatedTime
	if strings.TrimSpace(createdTime) == "" {
		createdTime = time.Now().UTC().Format("2006-01-02 15:04:05")
	}

	const insertSQL = `
		INSERT INTO deployment_information (
			deployment_node_id, jdk_version, core_count, deployment_info, deployment_info_hash, created_time, created_on, updated_on
		) VALUES ($1::uuid, $2, $3, $4::jsonb, $5, $6, NOW(), NOW())
		RETURNING id::text`

	var id string
	if err := r.db.QueryRow(ctx, insertSQL, nodeUUID, req.JDKVersion, req.CoreCount, string(deploymentInfoJSON), req.Hash, createdTime).Scan(&id); err != nil {
		return "", fmt.Errorf("insert deployment information: %w", err)
	}
	return id, nil
}

// InsertUsageStats records hourly usage and rolls up into daily_usage_summary.
func (r *instanceIngestionRepo) InsertUsageStats(ctx context.Context, req domain.InstanceUsageIngestRequest) error {
	ctx = ensureCallerIdentity(ctx)
	if strings.TrimSpace(req.NodeID) == "" {
		return &apierror.ValidationError{Msg: "nodeId is required"}
	}
	if strings.TrimSpace(req.CountType) == "" {
		return &apierror.ValidationError{Msg: "countType is required"}
	}

	var nodeUUID string
	err := r.db.QueryRow(ctx, "SELECT id::text FROM deployment_node WHERE node_id = $1 LIMIT 1", req.NodeID).Scan(&nodeUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			inserted, insErr := r.UpsertDeploymentNode(ctx, domain.InstanceMetaRequest{NodeID: req.NodeID})
			if insErr != nil {
				return fmt.Errorf("create missing deployment node for usage: %w", insErr)
			}
			nodeUUID = inserted
		} else {
			return fmt.Errorf("find deployment node: %w", err)
		}
	}

	countedOn := req.CreatedTime
	if strings.TrimSpace(countedOn) == "" {
		countedOn = time.Now().UTC().Format("2006-01-02 15:04:05")
	}

	// Insert hourly record
	const hourlySQL = `
		INSERT INTO hourly_usage_summary (deployment_node_id, count_type, count, counted_on, created_on, updated_on)
		VALUES ($1::uuid, $2, $3, $4::timestamp, NOW(), NOW())`
	if _, err := r.db.Exec(ctx, hourlySQL, nodeUUID, req.CountType, req.Count, countedOn); err != nil {
		return fmt.Errorf("insert hourly usage: %w", err)
	}

	// Extract summary date (YYYY-MM-DD)
	summaryDate := countedOn
	if len(summaryDate) >= 10 {
		summaryDate = summaryDate[:10]
	}

	// Upsert daily roll-up (API_CALL)
	var existingDailyID string
	err = r.db.QueryRow(ctx, `
		SELECT id::text FROM daily_usage_summary
		WHERE deployment_node_id = $1::uuid AND summary_date = $2::date AND count_type = $3 AND data_source = 'API_CALL'
		LIMIT 1`, nodeUUID, summaryDate, req.CountType).Scan(&existingDailyID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check daily usage: %w", err)
	}

	if existingDailyID != "" {
		const updateDailySQL = `UPDATE daily_usage_summary SET value = $2, updated_on = NOW() WHERE id = $1::uuid`
		if _, err := r.db.Exec(ctx, updateDailySQL, existingDailyID, req.Count); err != nil {
			return fmt.Errorf("update daily usage: %w", err)
		}
	} else {
		const insertDailySQL = `
			INSERT INTO daily_usage_summary (deployment_node_id, summary_date, count_type, value, data_source, created_on, updated_on)
			VALUES ($1::uuid, $2::date, $3, $4, 'API_CALL', NOW(), NOW())`
		if _, err := r.db.Exec(ctx, insertDailySQL, nodeUUID, summaryDate, req.CountType, req.Count); err != nil {
			return fmt.Errorf("insert daily usage: %w", err)
		}
	}

	return nil
}

// ImportDailyUsagesBatch batch-inserts daily records into daily_usage_summary.
func (r *instanceIngestionRepo) ImportDailyUsagesBatch(ctx context.Context, req domain.DailyUsageBatchRequest) (int, error) {
	ctx = ensureCallerIdentity(ctx)
	if strings.TrimSpace(req.NodeID) == "" {
		return 0, &apierror.ValidationError{Msg: "nodeId is required"}
	}

	nodeUUID, err := r.UpsertDeploymentNode(ctx, domain.InstanceMetaRequest{
		NodeID:           req.NodeID,
		ProjectKey:       req.ProjectKey,
		DeploymentNumber: req.DeploymentNumber,
		Product:          req.Product,
		Version:          req.Version,
	})
	if err != nil {
		return 0, fmt.Errorf("ensure deployment node: %w", err)
	}

	importedCount := 0
	for _, entry := range req.Usages {
		if strings.TrimSpace(entry.Date) == "" || strings.TrimSpace(entry.Type) == "" {
			continue
		}

		var existingID string
		err := r.db.QueryRow(ctx, `
			SELECT id::text FROM daily_usage_summary
			WHERE deployment_node_id = $1::uuid AND summary_date = $2::date AND count_type = $3 AND data_source = 'FILE_UPLOAD'
			LIMIT 1`, nodeUUID, entry.Date, entry.Type).Scan(&existingID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return importedCount, fmt.Errorf("check existing daily usage: %w", err)
		}

		if existingID != "" {
			const updateSQL = `
				UPDATE daily_usage_summary
				SET value = $2, created_by = $3, updated_on = NOW()
				WHERE id = $1::uuid`
			if _, err := r.db.Exec(ctx, updateSQL, existingID, entry.Value, req.UserEmail); err != nil {
				return importedCount, fmt.Errorf("update daily usage: %w", err)
			}
		} else {
			const insertSQL = `
				INSERT INTO daily_usage_summary (
					deployment_node_id, summary_date, count_type, value, data_source, created_by, created_on, updated_on
				) VALUES ($1::uuid, $2::date, $3, $4, 'FILE_UPLOAD', $5, NOW(), NOW())`
			if _, err := r.db.Exec(ctx, insertSQL, nodeUUID, entry.Date, entry.Type, entry.Value, req.UserEmail); err != nil {
				return importedCount, fmt.Errorf("insert daily usage: %w", err)
			}
		}
		importedCount++
	}

	return importedCount, nil
}

// GetProjectSecretKey retrieves product_consumption_primary_secret_key from project table.
func (r *instanceIngestionRepo) GetProjectSecretKey(ctx context.Context, projectKey string) (string, error) {
	ctx = ensureCallerIdentity(ctx)
	trimmed := strings.TrimSpace(projectKey)
	if trimmed == "" {
		return "", &apierror.ValidationError{Msg: "project key is required"}
	}

	const query = `
		SELECT COALESCE(product_consumption_primary_secret_key, '')
		FROM project
		WHERE key = $1 OR id::text = $1
		LIMIT 1`

	var secretKey string
	err := r.db.QueryRow(ctx, query, trimmed).Scan(&secretKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", &apierror.NotFoundError{Msg: fmt.Sprintf("project %q not found", trimmed)}
		}
		return "", fmt.Errorf("get project secret key: %w", err)
	}
	return secretKey, nil
}
