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

package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// stubRoundTripper returns a fixed JSON body for every request.
type stubRoundTripper struct {
	body string
}

func (s stubRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     make(http.Header),
	}, nil
}

func TestX5CStrippingTransport_RemovesX5CFromEveryKey(t *testing.T) {
	rawJWKS := `{"keys":[
		{"kty":"RSA","kid":"a","n":"abc","e":"AQAB","x5c":["garbage-cert-data"]},
		{"kty":"RSA","kid":"b","n":"def","e":"AQAB"}
	]}`

	transport := &x5cStrippingTransport{base: stubRoundTripper{body: rawJWKS}}
	resp, err := transport.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	defer resp.Body.Close()

	sanitized, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read sanitized body: %v", err)
	}

	var jwks struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(sanitized, &jwks); err != nil {
		t.Fatalf("sanitized body is not valid JSON: %v", err)
	}

	if len(jwks.Keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(jwks.Keys))
	}
	for _, key := range jwks.Keys {
		if _, present := key["x5c"]; present {
			t.Errorf("expected x5c to be stripped from key %v, but it is still present", key["kid"])
		}
		n, nOK := key["n"].(string)
		e, eOK := key["e"].(string)
		kid, kidOK := key["kid"].(string)
		if !nOK || n == "" || !eOK || e == "" || !kidOK || kid == "" {
			t.Errorf("expected other JWK fields to survive stripping, got %v", key)
		}
	}
}

// A valid, non-JWKS JSON body (e.g. an upstream error payload the JWKS endpoint
// returned with an unexpected 200 status) must survive untouched — not get silently
// replaced by a fabricated {"keys":null} document, which a naive
// unmarshal-into-Keys-then-remarshal would produce since json.Unmarshal doesn't error
// just because the source has no "keys" field.
func TestX5CStrippingTransport_PassesThroughNonJWKSResponse(t *testing.T) {
	nonJWKS := `{"error":"upstream unavailable","code":503}`

	transport := &x5cStrippingTransport{base: stubRoundTripper{body: nonJWKS}}
	resp, err := transport.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if string(got) != nonJWKS {
		t.Errorf("expected non-JWKS body to pass through unchanged, got %q", got)
	}
}
