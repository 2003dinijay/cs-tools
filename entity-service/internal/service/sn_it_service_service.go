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
	"encoding/json"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snITServicesResponse mirrors the Choreo POST /services/search response.
type snITServicesResponse struct {
	Services     []snITService `json:"services"`
	TotalRecords int           `json:"totalRecords"`
	Offset       int           `json:"offset"`
	Limit        int           `json:"limit"`
}

type snITService struct {
	ID                    string             `json:"id"`
	Name                  *string            `json:"name"`
	Class                 *string            `json:"class"`
	BusinessCriticality   *snITServiceLabel  `json:"businessCriticality"`
	ServiceClassification *snITServiceLabel  `json:"serviceClassification"`
}

type snITServiceLabel struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// snITServiceSearchPayload is the Choreo POST /services/search request body.
type snITServiceSearchPayload struct {
	Pagination snProjectPagination `json:"pagination"`
}

type snITServiceService struct {
	client *integrationservice.Client
}

// NewServiceNowITServiceService constructs an ITServiceService backed by the Choreo API.
func NewServiceNowITServiceService(client *integrationservice.Client) ITServiceService {
	return &snITServiceService{client: client}
}

// SearchITServices implements ITServiceService.
func (s *snITServiceService) SearchITServices(ctx context.Context, req domain.SearchITServicesRequest) (domain.SearchITServicesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchITServicesResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchITServicesResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snITServiceSearchPayload{
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}
	raw, err := s.client.Post(ctx, "/services/search", token, payload)
	if err != nil {
		return domain.SearchITServicesResponse{}, err
	}

	var snResp snITServicesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchITServicesResponse{}, fmt.Errorf("sn services: parse response: %w", err)
	}

	services := make([]domain.ITService, 0, len(snResp.Services))
	for _, svc := range snResp.Services {
		item := domain.ITService{
			ID:    sysidToUUID(svc.ID),
			Name:  svc.Name,
			Class: svc.Class,
		}
		if svc.BusinessCriticality != nil {
			item.BusinessCriticality = &domain.ITServiceLabelRef{
				ID:    svc.BusinessCriticality.ID,
				Label: svc.BusinessCriticality.Label,
			}
		}
		if svc.ServiceClassification != nil {
			item.ServiceClassification = &domain.ITServiceLabelRef{
				ID:    svc.ServiceClassification.ID,
				Label: svc.ServiceClassification.Label,
			}
		}
		services = append(services, item)
	}

	return domain.SearchITServicesResponse{
		Services: services,
		Total:    snResp.TotalRecords,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}, nil
}
