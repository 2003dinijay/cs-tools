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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func sampleMappings() []domain.ProductRepoMapping {
	return []domain.ProductRepoMapping{
		{ProductName: "WSO2 Identity Server", Owner: "wso2-enterprise", Repository: "wso2-iam-internal", GithubLabel: "IS"},
		{ProductName: "WSO2 Identity Server Analytics", Owner: "wso2-enterprise", Repository: "wso2-iam-internal", GithubLabel: "IS-Analytics"},
		{ProductName: "WSO2 API Manager", Abbreviation: strPtr("wso2am"), Owner: "wso2-enterprise", Repository: "wso2-apim-internal", GithubLabel: "APIM"},
		{ProductName: "BI", Owner: "wso2-enterprise", Repository: "wso2-integration-internal", GithubLabel: "BI"},
		{ProductName: "Bijira", Owner: "wso2-enterprise", Repository: "wso2-apim-internal", GithubLabel: "Bijira"},
	}
}

func TestMatchProductRepo(t *testing.T) {
	rows := sampleMappings()
	tests := []struct {
		name    string
		query   string
		want    string
		wantHit bool
	}{
		{name: "exact product name", query: "WSO2 Identity Server", want: "IS", wantHit: true},
		{name: "exact abbreviation", query: "wso2am", want: "APIM", wantHit: true},
		{name: "versioned name matches the product prefix", query: "WSO2 Identity Server 6.0.0", want: "IS", wantHit: true},
		{name: "longer prefix wins over the shorter one", query: "WSO2 Identity Server Analytics 1.2.0", want: "IS-Analytics", wantHit: true},
		{name: "bijira is not bi", query: "Bijira", want: "Bijira", wantHit: true},
		{name: "short name is not a prefix of the catalogue name", query: "Identity Server", wantHit: false},
		{name: "unknown product", query: "Not A Product", wantHit: false},
		{name: "blank", query: "  ", wantHit: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := MatchProductRepo(rows, tt.query)
			if ok != tt.wantHit {
				t.Fatalf("hit=%v want %v", ok, tt.wantHit)
			}
			if ok && got.GithubLabel != tt.want {
				t.Fatalf("label=%q want %q", got.GithubLabel, tt.want)
			}
		})
	}
}
