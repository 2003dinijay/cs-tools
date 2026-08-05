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

package sweep

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// pageSize is the page size used for /projects/search. entity-service's own
// code states maxLimit = 100 (entity-service/internal/service/user_service.go),
// but that does not match live behavior: confirmed via direct Postman
// testing against staging, limit: 50 returns 200 OK while limit: 51 returns
// 400 "Invalid request payload." — the real, live maximum is 50, not 100.
const pageSize = 50

// Run performs one full ACP evaluation pass over every open project,
// paginating through /projects/search. A single project's processProject
// failure is logged and recorded in Result.Failures, and the sweep
// continues — one project's problem must never block the rest. A failure
// fetching a page itself is fatal for the whole run (there is no way to
// know what projects exist beyond it) and is returned as a non-nil error;
// Result still reflects whatever was evaluated before that point.
//
// If projectID is non-empty, Run is scoped to exactly that one project — it
// fetches it directly via GetProject and never calls SearchProjects at all.
// This is what backs TEST_PROJECT_ID: a scoped run cannot accidentally touch
// every open project in an environment. A GetProject failure in this mode is
// fatal, the same as a page-fetch failure in the broad-sweep mode — there is
// nothing else to fall back to when the one requested project can't be
// fetched.
func Run(ctx context.Context, reader sweepReader, updater projectUpdater, ntf notifier, now time.Time, projectID string) (Result, error) {
	var result Result

	if projectID != "" {
		raw, err := reader.GetProject(ctx, projectID)
		if err != nil {
			return result, fmt.Errorf("sweep: get project %s: %w", projectID, err)
		}

		var proj project
		if err := json.Unmarshal(raw, &proj); err != nil {
			return result, fmt.Errorf("sweep: parse project %s: %w", projectID, err)
		}

		result.ProjectsEvaluated++
		if err := processProject(ctx, reader, updater, ntf, now, proj); err != nil {
			slog.ErrorContext(ctx, "processProject failed", "projectID", proj.ID, "err", err)
			result.Failures = append(result.Failures, ProjectFailure{ProjectID: proj.ID, Err: err})
		}

		return result, nil
	}

	offset := 0
	for {
		reqBody, err := json.Marshal(searchProjectsRequest{
			Pagination:    pagination{Limit: pageSize, Offset: offset},
			ClosureStatus: "Open",
			SortBy:        "endDate",
			SortOrder:     "asc",
		})
		if err != nil {
			return result, fmt.Errorf("sweep: build search request: %w", err)
		}

		raw, err := reader.SearchProjects(ctx, reqBody)
		if err != nil {
			return result, fmt.Errorf("sweep: search projects at offset %d: %w", offset, err)
		}

		var page searchProjectsResponse
		if err := json.Unmarshal(raw, &page); err != nil {
			return result, fmt.Errorf("sweep: parse search response at offset %d: %w", offset, err)
		}

		for _, proj := range page.Projects {
			result.ProjectsEvaluated++
			if err := processProject(ctx, reader, updater, ntf, now, proj); err != nil {
				slog.ErrorContext(ctx, "processProject failed", "projectID", proj.ID, "err", err)
				result.Failures = append(result.Failures, ProjectFailure{ProjectID: proj.ID, Err: err})
			}
		}

		if len(page.Projects) == 0 {
			break
		}

		if !page.HasMore {
			break
		}

		if page.Total > 0 && result.ProjectsEvaluated >= page.Total {
			slog.WarnContext(ctx, "sweep: pagination hit the Total bound while hasMore was still true",
				"total", page.Total, "projectsEvaluated", result.ProjectsEvaluated)
			break
		}

		offset += pageSize
	}

	return result, nil
}
