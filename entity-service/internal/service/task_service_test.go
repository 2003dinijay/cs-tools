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
// KIND, either express or implied. See the License for the
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

// The Postgres data source has no task store. Every task operation must report
// the documented 503 ServiceUnavailableError rather than the route being absent
// (which the mux would answer with an undocumented 404).
func TestUnavailableTaskService(t *testing.T) {
	svc := NewUnavailableTaskService()
	ctx := context.Background()

	calls := map[string]func() error{
		"SearchCaseTasks": func() error {
			_, err := svc.SearchCaseTasks(ctx, "11111111-1111-1111-1111-111111111111", domain.SearchCaseTasksRequest{})
			return err
		},
		"SearchTasks": func() error {
			_, err := svc.SearchTasks(ctx, domain.SearchTasksRequest{})
			return err
		},
		"GetTask": func() error {
			_, err := svc.GetTask(ctx, "11111111-1111-1111-1111-111111111111")
			return err
		},
		"CreateCaseTask": func() error {
			_, err := svc.CreateCaseTask(ctx, "11111111-1111-1111-1111-111111111111", domain.CreateCaseTaskRequest{})
			return err
		},
		"UpdateTask": func() error {
			_, err := svc.UpdateTask(ctx, "11111111-1111-1111-1111-111111111111", domain.UpdateTaskRequest{})
			return err
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()
			var sue *apierror.ServiceUnavailableError
			if !errors.As(err, &sue) {
				t.Fatalf("err = %v (%T), want *apierror.ServiceUnavailableError", err, err)
			}
			if sue.Msg != taskUnavailableMsg {
				t.Errorf("Msg = %q, want %q", sue.Msg, taskUnavailableMsg)
			}
		})
	}
}
