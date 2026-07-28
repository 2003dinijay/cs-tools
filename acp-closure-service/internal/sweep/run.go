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

const pageSize = 100

// Run performs one full ACP evaluation pass over every open project,
// paginating through /projects/search. A single project's processProject
// failure is logged and recorded in Result.Failures, and the sweep
// continues — one project's problem must never block the rest. A failure
// fetching a page itself is fatal for the whole run (there is no way to
// know what projects exist beyond it) and is returned as a non-nil error;
// Result still reflects whatever was evaluated before that point.
func Run(ctx context.Context, reader sweepReader, updater projectUpdater, ntf notifier, now time.Time) (Result, error) {
	var result Result

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

		if !page.HasMore {
			break
		}
		offset += pageSize
	}

	return result, nil
}
