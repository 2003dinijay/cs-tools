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
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// DeploymentAirGapRepository sets a deployment's air-gapped flag (migration 0246).
// It is Postgres-only: ServiceNow has no such field, so nothing is mirrored back.
type DeploymentAirGapRepository interface {
	SetAirGapped(ctx context.Context, id string, airGapped bool, updatedBy string) (domain.DeploymentAirGapped, error)
}

type deploymentAirGapRepo struct {
	db *Scoped
}

// NewDeploymentAirGapRepository constructs a DeploymentAirGapRepository backed by *Scoped.
func NewDeploymentAirGapRepository(db *Scoped) DeploymentAirGapRepository {
	return &deploymentAirGapRepo{db: db}
}

// SetAirGapped implements DeploymentAirGapRepository. An unknown deployment is a
// NotFoundError. Setting the value it already has still stamps updated_on/updated_by,
// so the last person to confirm it is on record.
func (r *deploymentAirGapRepo) SetAirGapped(ctx context.Context, id string, airGapped bool, updatedBy string) (domain.DeploymentAirGapped, error) {
	var out domain.DeploymentAirGapped
	err := r.db.QueryRow(ctx, `
		UPDATE deployment
		SET air_gapped = $2, updated_on = NOW(), updated_by = $3
		WHERE id = $1::uuid
		RETURNING id::text, number, air_gapped, updated_on, updated_by`, id, airGapped, updatedBy).
		Scan(&out.ID, &out.Number, &out.AirGapped, &out.UpdatedOn, &out.UpdatedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DeploymentAirGapped{}, &apierror.NotFoundError{Msg: fmt.Sprintf("deployment %q not found", id)}
		}
		return domain.DeploymentAirGapped{}, fmt.Errorf("set deployment air-gapped: %w", err)
	}
	return out, nil
}
