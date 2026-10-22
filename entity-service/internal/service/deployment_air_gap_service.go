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
	"context"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// airGapClientActor is stamped as updated_by when the flag is set by a service with
// no end user behind the call.
const airGapClientActor = "internal-client"

// DeploymentAirGapService marks deployments air-gapped or not. Who may do it is
// decided before this: the route is internal-only, and a portal backend that exposes
// it limits it further to the staff group allowed to.
type DeploymentAirGapService interface {
	SetAirGapped(ctx context.Context, id string, req domain.SetDeploymentAirGappedRequest) (domain.DeploymentAirGapped, error)
}

type deploymentAirGapService struct {
	repo repository.DeploymentAirGapRepository
}

// NewDeploymentAirGapService constructs a DeploymentAirGapService.
func NewDeploymentAirGapService(repo repository.DeploymentAirGapRepository) DeploymentAirGapService {
	return &deploymentAirGapService{repo: repo}
}

// SetAirGapped implements DeploymentAirGapService. The change is attributed to the
// end user when the call carries their x-user-id-token, and to the calling service
// otherwise.
func (s *deploymentAirGapService) SetAirGapped(ctx context.Context, id string, req domain.SetDeploymentAirGappedRequest) (domain.DeploymentAirGapped, error) {
	id = strings.TrimSpace(id)
	if err := validateUUID("id", id); err != nil {
		return domain.DeploymentAirGapped{}, err
	}
	if req.AirGapped == nil {
		return domain.DeploymentAirGapped{}, &apierror.ValidationError{Msg: "airGapped is required"}
	}
	actor := airGapClientActor
	if token := middleware.UserIDTokenFromContext(ctx); token != "" {
		email, err := emailFromJWT(token)
		if err != nil {
			return domain.DeploymentAirGapped{}, &apierror.ValidationError{Msg: "x-user-id-token: " + err.Error()}
		}
		actor = email
	}
	return s.repo.SetAirGapped(ctx, id, *req.AirGapped, actor)
}
