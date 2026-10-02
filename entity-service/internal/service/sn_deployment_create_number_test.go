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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	testDeploySysid = "0123456789abcdef0123456789abcdef"
	testDeployProj  = "11111111-2222-3333-4444-555555555555"
)

func testCreateDeploymentReq() domain.CreateDeploymentRequest {
	typ := domain.DeploymentTypeDevelopment
	return domain.CreateDeploymentRequest{ProjectID: testDeployProj, Name: "Dev One", Type: &typ, Description: "d"}
}

// searchHandler counts /deployments/search calls and replies with searchBody
// (or status 500 when searchStatus != 0). POST /deployments replies createBody.
func deploymentCreateHandler(createBody string, searchBody string, searchStatus int, searches *int32) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/deployments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(createBody))
	})
	mux.HandleFunc("/deployments/search", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(searches, 1)
		bodyBytes, _ := io.ReadAll(r.Body)
		var body snDeploymentSearchPayload
		_ = json.Unmarshal(bodyBytes, &body)
		var raw struct {
			Filters map[string]json.RawMessage `json:"filters"`
		}
		_ = json.Unmarshal(bodyBytes, &raw)
		for k := range raw.Filters {
			if k != "projectIds" && k != "sfIds" {
				http.Error(w, `{"message":"field 'filters.`+k+`' cannot be added to the closed record"}`, http.StatusBadRequest)
				return
			}
		}
		if body.Pagination.Limit < 1 || body.Pagination.Limit > 50 {
			http.Error(w, `{"message":"Invalid Pagination Limit. Must be between 1 and 50."}`, http.StatusBadRequest)
			return
		}
		if searchStatus != 0 {
			http.Error(w, `{"message":"boom"}`, searchStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(searchBody))
	})
	return mux
}

func createReply(number string) string {
	d := map[string]string{"id": testDeploySysid, "createdOn": "2020-01-01 00:00:00", "createdBy": "jane.doe@example.com"}
	if number != "" {
		d["number"] = number
	}
	b, _ := json.Marshal(map[string]any{"message": "ok", "deployment": d})
	return string(b)
}

func searchReply(number string) string {
	b, _ := json.Marshal(map[string]any{
		"deployments":  []map[string]any{{"id": "ffffffffffffffffffffffffffffffff", "number": "OTHER"}, {"id": testDeploySysid, "number": number}},
		"totalRecords": 2,
	})
	return string(b)
}

func TestCreateDeploymentSNFirstDetails_NumberInReply_NoFetch(t *testing.T) {
	var searches int32
	svc := &snDeploymentService{client: newTestSNClient(t, deploymentCreateHandler(createReply("DEP0001"), "", 0, &searches))}
	id, number, by, _, err := svc.createDeploymentSNFirstDetails(t.Context(), testCreateDeploymentReq())
	if err != nil {
		t.Fatal(err)
	}
	if number != "DEP0001" || id != sysidToUUID(testDeploySysid) || by != "jane.doe@example.com" {
		t.Fatalf("got id=%s number=%s by=%s", id, number, by)
	}
	if searches != 0 {
		t.Fatalf("expected no fetch, got %d", searches)
	}
}

func TestCreateDeploymentSNFirstDetails_NoNumber_FetchesByID(t *testing.T) {
	var searches int32
	svc := &snDeploymentService{client: newTestSNClient(t, deploymentCreateHandler(createReply(""), searchReply("DEP0042"), 0, &searches))}
	_, number, _, _, err := svc.createDeploymentSNFirstDetails(t.Context(), testCreateDeploymentReq())
	if err != nil {
		t.Fatal(err)
	}
	if number != "DEP0042" {
		t.Fatalf("number = %q, want DEP0042", number)
	}
	if searches != 1 {
		t.Fatalf("expected exactly one fetch, got %d", searches)
	}
}

func TestCreateDeploymentSNFirstDetails_FetchHasNoNumber_ValidationError(t *testing.T) {
	var searches int32
	svc := &snDeploymentService{client: newTestSNClient(t, deploymentCreateHandler(createReply(""), searchReply(""), 0, &searches))}
	_, _, _, _, err := svc.createDeploymentSNFirstDetails(t.Context(), testCreateDeploymentReq())
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "response number is required") {
		t.Fatalf("want ValidationError, got %v", err)
	}
}

func TestCreateDeploymentSNFirstDetails_FetchErrors_WrapsID(t *testing.T) {
	var searches int32
	svc := &snDeploymentService{client: newTestSNClient(t, deploymentCreateHandler(createReply(""), "", http.StatusInternalServerError, &searches))}
	_, _, _, _, err := svc.createDeploymentSNFirstDetails(t.Context(), testCreateDeploymentReq())
	if err == nil || !strings.Contains(err.Error(), testDeploySysid) {
		t.Fatalf("want error containing id %s, got %v", testDeploySysid, err)
	}
}

func TestCreateDeploymentSNFirstDetails_NumberOnSecondPage(t *testing.T) {
	var searches int32
	mux := http.NewServeMux()
	mux.HandleFunc("/deployments", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(createReply("")))
	})
	mux.HandleFunc("/deployments/search", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&searches, 1)
		bodyBytes, _ := io.ReadAll(r.Body)
		var body snDeploymentSearchPayload
		_ = json.Unmarshal(bodyBytes, &body)
		var raw struct {
			Filters map[string]json.RawMessage `json:"filters"`
		}
		_ = json.Unmarshal(bodyBytes, &raw)
		for k := range raw.Filters {
			if k != "projectIds" && k != "sfIds" {
				http.Error(w, `{"message":"field 'filters.`+k+`' cannot be added to the closed record"}`, http.StatusBadRequest)
				return
			}
		}
		if body.Pagination.Limit < 1 || body.Pagination.Limit > 50 {
			http.Error(w, `{"message":"Invalid Pagination Limit. Must be between 1 and 50."}`, http.StatusBadRequest)
			return
		}
		rows := []map[string]any{}
		if body.Pagination.Offset == 0 {
			for i := 0; i < 50; i++ {
				rows = append(rows, map[string]any{"id": fmt.Sprintf("%032d", i+1), "number": "X"})
			}
		} else {
			rows = append(rows, map[string]any{"id": testDeploySysid, "number": "DEP0099"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"deployments": rows, "totalRecords": 51})
	})
	svc := &snDeploymentService{client: newTestSNClient(t, mux)}
	_, number, _, _, err := svc.createDeploymentSNFirstDetails(t.Context(), testCreateDeploymentReq())
	if err != nil {
		t.Fatal(err)
	}
	if number != "DEP0099" || searches != 2 {
		t.Fatalf("number=%q searches=%d, want DEP0099 and 2", number, searches)
	}
}

func TestCreateDeploymentSNFirstDetails_CreatedOnIsNowNotReply(t *testing.T) {
	var searches int32
	// createReply's createdOn is 2020-01-01 00:00:00; it must be ignored.
	svc := &snDeploymentService{client: newTestSNClient(t, deploymentCreateHandler(createReply("DEP0001"), "", 0, &searches))}
	before := time.Now().UTC()
	_, _, _, createdOn, err := svc.createDeploymentSNFirstDetails(t.Context(), testCreateDeploymentReq())
	after := time.Now().UTC()
	if err != nil {
		t.Fatal(err)
	}
	if createdOn.Before(before) || createdOn.After(after) || createdOn.Location() != time.UTC {
		t.Fatalf("createdOn = %v (%v), want UTC within [%v, %v]", createdOn, createdOn.Location(), before, after)
	}
}
