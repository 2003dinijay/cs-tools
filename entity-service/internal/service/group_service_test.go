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

type recordingGroupRepo struct {
	query          string
	assignableOnly bool
	calls          int
}

func (r *recordingGroupRepo) SearchGroups(_ context.Context, query string, assignableOnly bool, _, _ int) ([]domain.Group, int, error) {
	r.calls++
	r.query, r.assignableOnly = query, assignableOnly
	return []domain.Group{}, 0, nil
}

// assignableOnly is what the assignment-group picker sends; every other caller
// (the users page's team filters) must keep seeing the whole registry.
func TestGroupService_SearchGroups_AssignableOnlyReachesTheRepository(t *testing.T) {
	cases := map[string]struct {
		filters *domain.SearchGroupsFilters
		want    bool
	}{
		"no filters":          {nil, false},
		"query only":          {&domain.SearchGroupsFilters{SearchQuery: "ap"}, false},
		"assignableOnly":      {&domain.SearchGroupsFilters{SearchQuery: "ap", AssignableOnly: true}, true},
		"assignableOnly bare": {&domain.SearchGroupsFilters{AssignableOnly: true}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &recordingGroupRepo{}
			_, err := NewGroupService(repo).SearchGroups(context.Background(), domain.SearchGroupsRequest{Filters: tc.filters})
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if repo.calls != 1 || repo.assignableOnly != tc.want {
				t.Errorf("repository got assignableOnly = %v (calls %d), want %v", repo.assignableOnly, repo.calls, tc.want)
			}
		})
	}
}
