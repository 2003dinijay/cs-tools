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

package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// tokenRecorder is a GitHub stand-in that records the token each request
// carried, keyed by "METHOD path".
type tokenRecorder struct {
	mu   sync.Mutex
	seen map[string]string
}

func newTokenRecorder(t *testing.T) (*tokenRecorder, *httptest.Server) {
	rec := &tokenRecorder{seen: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.seen[r.Method+" "+r.URL.Path] = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		rec.mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/dispatches"):
			w.WriteHeader(http.StatusNoContent)
		case strings.Contains(r.URL.Path, "/contents/"):
			_, _ = w.Write([]byte("case:\n  account: a\n"))
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":1,"number":7,"html_url":"https://github.com/x/y/issues/7"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return rec, srv
}

func (r *tokenRecorder) token(key string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.seen[key]
}

func TestRouter_UsesEachRepositorysOwnToken(t *testing.T) {
	rec, srv := newTokenRecorder(t)
	tokens, err := ParseRepoTokens(`{"wso2-enterprise/choreo":"pat-choreo","Wso2-Enterprise/WSO2Cloud":"pat-cloud","asgardeo-org":"pat-asgardeo"}`)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRouter(Config{BaseURL: srv.URL}, "pat-fallback", tokens)
	ctx := context.Background()

	if err := r.Dispatch(ctx, "wso2-enterprise", "choreo", "servicenow-note", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	// Case-insensitive, as GitHub routes, both in the key and in the call.
	if _, err := r.FileContent(ctx, "WSO2-Enterprise", "wso2cloud", ".github/servicenow-config.yml"); err != nil {
		t.Fatal(err)
	}
	// An owner-level key covers every repository of that owner.
	if _, err := r.CreateComment(ctx, Issue{Owner: "asgardeo-org", Repository: "product", Number: 7}, "hi"); err != nil {
		t.Fatal(err)
	}
	// Anything else falls back to GITHUB_TOKEN.
	if err := r.AddLabel(ctx, Issue{Owner: "SasmithaDilshan", Repository: "test-cr-repo", Number: 7}, "Status/Assigned"); err != nil {
		t.Fatal(err)
	}

	for key, want := range map[string]string{
		"POST /repos/wso2-enterprise/choreo/dispatches":                               "pat-choreo",
		"GET /repos/WSO2-Enterprise/wso2cloud/contents/.github/servicenow-config.yml": "pat-cloud",
		"POST /repos/asgardeo-org/product/issues/7/comments":                          "pat-asgardeo",
		"POST /repos/SasmithaDilshan/test-cr-repo/issues/7/labels":                    "pat-fallback",
	} {
		if got := rec.token(key); got != want {
			t.Errorf("%s sent token %q, want %q", key, got, want)
		}
	}
}

// With no token for a repository, every call fails -- and FileContent fails
// rather than returning (nil, nil), which would read as "no config file" and
// skip the repository's issues with no error.
func TestRouter_NoTokenForRepositoryIsAnError(t *testing.T) {
	_, srv := newTokenRecorder(t)
	r := NewRouter(Config{BaseURL: srv.URL}, "", map[string]string{"wso2-enterprise/choreo": "pat-choreo"})
	ctx := context.Background()

	body, err := r.FileContent(ctx, "other", "repo", ".github/servicenow-config.yml")
	if !errors.Is(err, ErrNoToken) || body != nil {
		t.Errorf("FileContent = (%q, %v), want ErrNoToken", body, err)
	}
	if err := r.Dispatch(ctx, "other", "repo", "servicenow-note", nil); !errors.Is(err, ErrNoToken) {
		t.Errorf("Dispatch err = %v, want ErrNoToken", err)
	}
	if _, err := r.CreateIssue(ctx, "other", "repo", "t", "b", nil); !errors.Is(err, ErrNoToken) {
		t.Errorf("CreateIssue err = %v, want ErrNoToken", err)
	}
	if err := r.SetState(ctx, Issue{Owner: "other", Repository: "repo", Number: 1}, StateClosed); !errors.Is(err, ErrNoToken) {
		t.Errorf("SetState err = %v, want ErrNoToken", err)
	}
}

func TestParseRepoTokens(t *testing.T) {
	if got, err := ParseRepoTokens("  "); err != nil || len(got) != 0 {
		t.Errorf("empty: got %v, %v; want no tokens, no error", got, err)
	}
	got, err := ParseRepoTokens(`{" Acme/One ":" t1 ","beta":"t2"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got["acme/one"] != "t1" || got["beta"] != "t2" || len(got) != 2 {
		t.Errorf("parsed %v, want trimmed, lower-cased keys", got)
	}
	for name, raw := range map[string]string{
		"not JSON":          `acme/one=t1`,
		"array":             `["t1"]`,
		"too many segments": `{"a/b/c":"t1"}`,
		"empty owner":       `{"/repo":"t1"}`,
		"empty repository":  `{"owner/":"t1"}`,
		"empty key":         `{" ":"t1"}`,
		"empty token":       `{"acme/one":"  "}`,
		"duplicate by case": `{"Acme/One":"t1","acme/one":"t2"}`,
	} {
		if _, err := ParseRepoTokens(raw); err == nil {
			t.Errorf("%s: want an error", name)
		} else if strings.Contains(err.Error(), "t1") || strings.Contains(err.Error(), "t2") {
			t.Errorf("%s: error %q quotes a token", name, err)
		}
	}
}
