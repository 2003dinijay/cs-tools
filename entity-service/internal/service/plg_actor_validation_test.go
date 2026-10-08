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

// TestAttributedWritesRefuseAMissingActor pins the rule validateActor exists for.
//
// An omitted actorId is not a loud failure on its own: it reaches uuidArg as "",
// which becomes a NULL column, so the write SUCCEEDS and records nobody. That is
// the worst possible outcome for an audit column — it looks like working
// attribution until someone asks who did something and the answer is blank.
//
// The pairing writes have always validated it. These four were added later and
// did not, which made the same request 400 on one route and 200-with-no-actor on
// another. This asserts the four, so a fifth attributed write added later fails
// here rather than silently joining the wrong side.
func TestAttributedWritesRefuseAMissingActor(t *testing.T) {
	const notAUUID = "jane.doe@example.com"

	cases := []struct {
		name string
		call func(actor string) error
	}{
		{"playbook create", func(a string) error {
			_, err := (&playbookService{}).Create(context.Background(), domain.CreatePlaybookRequest{
				ProductCode: "IAM", Name: "x",
				LifecycleStage: domain.StageActivated, PlaybookType: domain.PlaybookRecovery,
			}, a)
			return err
		}},
		{"playbook patch", func(a string) error {
			_, err := (&playbookService{}).Patch(context.Background(), domain.PatchPlaybookRequest{
				ID: "11111111-1111-1111-1111-111111111111",
			}, a)
			return err
		}},
		{"playbook replace tasks", func(a string) error {
			_, err := (&playbookService{}).ReplaceTasks(context.Background(), domain.ReplacePlaybookTasksRequest{
				PlaybookID: "11111111-1111-1111-1111-111111111111",
			}, a)
			return err
		}},
	}

	for _, c := range cases {
		for _, actor := range []string{"", notAUUID} {
			label := "empty"
			if actor != "" {
				label = "not a uuid"
			}
			t.Run(c.name+"/"+label, func(t *testing.T) {
				err := c.call(actor)
				if err == nil {
					t.Fatalf("an attributed write accepted actorId %q — it would record nobody", actor)
				}
				var ve *apierror.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("want a ValidationError the caller can act on, got %T: %v", err, err)
				}
			})
		}
	}
}
