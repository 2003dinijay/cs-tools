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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestClampCatalogPagination separates the echoed page size from the slice length. Returning
// the length as the limit told a caller advancing offset by limit that the page size had
// shrunk on the last (or only) page.
func TestClampCatalogPagination(t *testing.T) {
	tests := []struct {
		name                           string
		in                             domain.Pagination
		total                          int
		wantOffset, wantLimit, wantLen int
	}{
		{"requested limit exceeds total", domain.Pagination{Limit: 50}, 10, 0, 50, 10},
		{"full page", domain.Pagination{Limit: 5}, 10, 0, 5, 5},
		{"last partial page", domain.Pagination{Limit: 5, Offset: 8}, 10, 8, 5, 2},
		{"unset limit defaults", domain.Pagination{}, 10, 0, catalogDefaultLimit, 10},
		{"limit above cap is clamped", domain.Pagination{Limit: 5000}, 10, 0, catalogMaxLimit, 10},
		{"negative offset floors at zero", domain.Pagination{Limit: 5, Offset: -3}, 10, 0, 5, 5},
		{"offset past total yields empty page", domain.Pagination{Limit: 5, Offset: 99}, 10, 10, 5, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			offset, limit, length := clampCatalogPagination(tc.in, tc.total)
			if offset != tc.wantOffset || limit != tc.wantLimit || length != tc.wantLen {
				t.Errorf("clampCatalogPagination(%+v, %d) = (%d, %d, %d), want (%d, %d, %d)",
					tc.in, tc.total, offset, limit, length, tc.wantOffset, tc.wantLimit, tc.wantLen)
			}
		})
	}
}

// TestSearchRoles_ReportsRequestedLimit is the regression this exists for: a catalogue
// smaller than the page size must still report the effective page size, matching every
// other search in this package.
func TestSearchRoles_ReportsRequestedLimit(t *testing.T) {
	got, err := NewRoleService().SearchRoles(context.Background(), domain.SearchRolesRequest{
		Pagination: domain.Pagination{Limit: 50},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Limit != 50 {
		t.Errorf("Limit = %d, want 50 (the requested page size, not the page length)", got.Limit)
	}
	if got.Total != len(got.Roles) {
		t.Errorf("Total = %d but returned %d roles; the whole catalogue fits one page", got.Total, len(got.Roles))
	}
}
