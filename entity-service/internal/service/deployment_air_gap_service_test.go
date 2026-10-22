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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeAirGapRepo struct {
	calls     int
	id, actor string
	airGapped bool
}

func (f *fakeAirGapRepo) SetAirGapped(_ context.Context, id string, airGapped bool, updatedBy string) (domain.DeploymentAirGapped, error) {
	f.calls++
	f.id, f.airGapped, f.actor = id, airGapped, updatedBy
	return domain.DeploymentAirGapped{ID: id, AirGapped: airGapped, UpdatedBy: updatedBy}, nil
}

const airGapDeploymentID = "b206552b-1bd7-7e14-a002-c9d3604bcbda"

func TestDeploymentAirGap_RejectsBadInputBeforeTheRepository(t *testing.T) {
	yes := true
	for _, tc := range []struct {
		name string
		id   string
		req  domain.SetDeploymentAirGappedRequest
	}{
		{"not a uuid", "DEP000001136", domain.SetDeploymentAirGappedRequest{AirGapped: &yes}},
		{"no airGapped", airGapDeploymentID, domain.SetDeploymentAirGappedRequest{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeAirGapRepo{}
			_, err := NewDeploymentAirGapService(repo).SetAirGapped(context.Background(), tc.id, tc.req)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want a ValidationError, got %v", err)
			}
			if repo.calls != 0 {
				t.Errorf("a rejected request reached the repository")
			}
		})
	}
}

func TestDeploymentAirGap_AttributesTheChange(t *testing.T) {
	yes, no := true, false

	repo := &fakeAirGapRepo{}
	if _, err := NewDeploymentAirGapService(repo).SetAirGapped(context.Background(), " "+airGapDeploymentID+" ", domain.SetDeploymentAirGappedRequest{AirGapped: &yes}); err != nil {
		t.Fatal(err)
	}
	if repo.id != airGapDeploymentID || !repo.airGapped || repo.actor != airGapClientActor {
		t.Errorf("service call without a user: id %q airGapped %v actor %q", repo.id, repo.airGapped, repo.actor)
	}

	repo = &fakeAirGapRepo{}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "csm.user@wso2.com"))
	if _, err := NewDeploymentAirGapService(repo).SetAirGapped(ctx, airGapDeploymentID, domain.SetDeploymentAirGappedRequest{AirGapped: &no}); err != nil {
		t.Fatal(err)
	}
	if repo.airGapped || repo.actor != "csm.user@wso2.com" {
		t.Errorf("call on behalf of a user: airGapped %v actor %q", repo.airGapped, repo.actor)
	}
}
