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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// maxEngagementAllocationUsers bounds how many people one read may ask about.
const maxEngagementAllocationUsers = 500

// SearchEngagementAllocations returns the confirmed customer engagement
// allocations of the requested people over the window. Internal callers only,
// like the rest of the team schedule.
func (s *scheduleService) SearchEngagementAllocations(ctx context.Context, req domain.SearchScheduleEngagementAllocationsRequest) (domain.ScheduleEngagementAllocationsResponse, error) {
	if err := s.requireInternalCaller(ctx); err != nil {
		return domain.ScheduleEngagementAllocationsResponse{}, err
	}
	if _, _, err := parseWindow(req.From, req.To); err != nil {
		return domain.ScheduleEngagementAllocationsResponse{}, err
	}
	if len(req.UserIDs) == 0 {
		return domain.ScheduleEngagementAllocationsResponse{}, &apierror.ValidationError{Msg: "userIds is required"}
	}
	if len(req.UserIDs) > maxEngagementAllocationUsers {
		return domain.ScheduleEngagementAllocationsResponse{}, &apierror.ValidationError{Msg: "too many userIds"}
	}
	for _, id := range req.UserIDs {
		if id == "" {
			return domain.ScheduleEngagementAllocationsResponse{}, &apierror.ValidationError{Msg: "userIds must not contain an empty id"}
		}
		if err := validateUserID(id); err != nil {
			return domain.ScheduleEngagementAllocationsResponse{}, err
		}
	}
	rows, err := s.repo.SearchEngagementAllocations(ctx, req)
	if err != nil {
		return domain.ScheduleEngagementAllocationsResponse{}, err
	}
	if rows == nil {
		rows = []domain.ScheduleEngagementAllocation{}
	}
	return domain.ScheduleEngagementAllocationsResponse{Allocations: rows, Count: len(rows)}, nil
}
