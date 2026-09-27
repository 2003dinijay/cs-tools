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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestIncidentWhereClause_CorrelationIDExactMatch(t *testing.T) {
	id := "fp:abc123:169000"
	where, args := incidentWhereClause(domain.SearchIncidentsFilters{CorrelationID: &id}, nil, nil, nil, nil, nil, nil, nil, nil)

	if !strings.Contains(where, "inc.correlation_id = $1") {
		t.Fatalf("expected an exact-match correlation_id clause, got: %s", where)
	}
	if len(args) != 1 || args[0] != id {
		t.Fatalf("expected args [%q], got %v", id, args)
	}
}

func TestIncidentWhereClause_CorrelationIDOmittedWhenAbsentOrEmpty(t *testing.T) {
	for _, id := range []*string{nil, strPtr("")} {
		where, args := incidentWhereClause(domain.SearchIncidentsFilters{CorrelationID: id}, nil, nil, nil, nil, nil, nil, nil, nil)
		if strings.Contains(where, "correlation_id") {
			t.Fatalf("expected no correlation_id clause for %v, got: %s", id, where)
		}
		if len(args) != 0 {
			t.Fatalf("expected no args for %v, got %v", id, args)
		}
	}
}
