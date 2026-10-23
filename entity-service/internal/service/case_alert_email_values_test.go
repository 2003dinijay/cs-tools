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
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// teamLookupFails answers every team lookup with an error, as a database
// that is down would.
type teamLookupFails struct{ *fakeCaseAlertRepo }

func (teamLookupFails) RosteredTeamFamily(context.Context, string) (string, bool, error) {
	return "", false, errors.New("lookup failed")
}

// A bad address or team key is refused as bad input before any team is
// looked up, so a failing lookup can never hide it.
func TestCaseAlertEmailTeamListsCheckedBeforeLookup(t *testing.T) {
	svc, _ := newCaseAlerts(newFakeCaseAlertRepo(), testCaseAlertKey)
	svc.repo = teamLookupFails{newFakeCaseAlertRepo()}
	for name, raw := range map[string]string{
		"bad address":     `{"atlas": "atlas team"}`,
		"bad team key":    `{"Atlas Team!": "atlas@example.com"}`,
		"same team twice": `{"atlas": "a@example.com", "ATLAS": "b@example.com"}`,
	} {
		_, err := svc.checkCaseAlertValue(context.Background(), "CRE", "email.teamLists", json.RawMessage(raw))
		wantAPIError[*apierror.ValidationError](t, err, name)
	}
	// Good input does reach the lookup, and its failure is not a refusal.
	_, err := svc.checkCaseAlertValue(context.Background(), "CRE", "email.teamLists", json.RawMessage(`{"atlas": "atlas@example.com"}`))
	var bad *apierror.ValidationError
	if err == nil || errors.As(err, &bad) {
		t.Fatalf("a failed lookup came back as %v, want a lookup error", err)
	}
}
