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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type stubSLAStatusRepo struct {
	search func(ctx context.Context, p domain.Pagination) ([]domain.SLAStatus, int, error)
}

func (s stubSLAStatusRepo) SearchActiveSLAStatuses(ctx context.Context, p domain.Pagination) ([]domain.SLAStatus, int, error) {
	return s.search(ctx, p)
}

// TestSLAStatusService_SearchActiveSLAStatuses proves the pagination default
// and cap are this endpoint's own (500/2000), not the generic 20/50 every
// other search uses — this is a machine-polling endpoint with one real
// caller, not a UI list.
func TestSLAStatusService_SearchActiveSLAStatuses(t *testing.T) {
	t.Run("no limit defaults to 500", func(t *testing.T) {
		var got domain.Pagination
		repo := stubSLAStatusRepo{search: func(_ context.Context, p domain.Pagination) ([]domain.SLAStatus, int, error) {
			got = p
			return nil, 0, nil
		}}
		if _, err := NewSLAStatusService(repo).SearchActiveSLAStatuses(context.Background(), domain.Pagination{}); err != nil {
			t.Fatal(err)
		}
		if got.Limit != defaultSLAStatusLimit {
			t.Errorf("limit = %d, want %d", got.Limit, defaultSLAStatusLimit)
		}
	})

	t.Run("limit above 2000 is rejected before reaching the repository", func(t *testing.T) {
		repo := stubSLAStatusRepo{search: func(context.Context, domain.Pagination) ([]domain.SLAStatus, int, error) {
			t.Fatal("repository must not be reached for an invalid limit")
			return nil, 0, nil
		}}
		_, err := NewSLAStatusService(repo).SearchActiveSLAStatuses(context.Background(), domain.Pagination{Limit: 2001})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("err = %v, want *apierror.ValidationError", err)
		}
	})

	t.Run("result is passed through with the normalized pagination echoed back", func(t *testing.T) {
		want := []domain.SLAStatus{{CaseID: "c1", ClockType: "response"}}
		repo := stubSLAStatusRepo{search: func(_ context.Context, p domain.Pagination) ([]domain.SLAStatus, int, error) {
			return want, 7, nil
		}}
		resp, err := NewSLAStatusService(repo).SearchActiveSLAStatuses(context.Background(), domain.Pagination{Limit: 10, Offset: 20})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Total != 7 || resp.Limit != 10 || resp.Offset != 20 || len(resp.Statuses) != 1 || resp.Statuses[0].CaseID != "c1" {
			t.Errorf("resp = %+v", resp)
		}
	})
}
