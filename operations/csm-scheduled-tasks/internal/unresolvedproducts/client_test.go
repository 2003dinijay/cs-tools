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
package unresolvedproducts

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(tokenSrv.Close)
	apiSrv := httptest.NewServer(handler)
	t.Cleanup(apiSrv.Close)
	c, err := NewClient(Config{BaseURL: apiSrv.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClient_ListSinceSendsTheWindowAndDecodes(t *testing.T) {
	var path, since, limit, auth string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path, since, limit, auth = r.URL.Path, r.URL.Query().Get("since"), r.URL.Query().Get("limit"), r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"count":1,"nodes":[{"nodeId":"n1","reportedProduct":"wso2zz","reportedVersion":"1.0.0","unresolvedSince":"2026-10-21T01:00:00Z","lastSeenOn":"2026-10-21T02:00:00Z"}],"truncated":true}`))
	})
	listing, err := c.ListSince(context.Background(), time.Date(2026, 10, 21, 6, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	nodes := listing.Nodes
	if !listing.Truncated || !nodes[0].UnresolvedSince.Equal(time.Date(2026, 10, 21, 1, 0, 0, 0, time.UTC)) {
		t.Errorf("listing %+v", listing)
	}
	if path != "/instances/unresolved-products" || since != "2026-10-21T06:00:00Z" || limit != "5000" || auth != "Bearer test-token" {
		t.Errorf("path %q since %q limit %q auth %q", path, since, limit, auth)
	}
	if len(nodes) != 1 || nodes[0].ReportedProduct != "wso2zz" || nodes[0].ReportedVersion == nil || *nodes[0].ReportedVersion != "1.0.0" {
		t.Errorf("nodes %+v", nodes)
	}
}

func TestClient_NonSuccessIsAnAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
	})
	_, err := c.ListSince(context.Background(), time.Now())
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("error = %v", err)
	}
}
