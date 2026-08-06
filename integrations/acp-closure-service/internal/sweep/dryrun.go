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
	"log/slog"
)

// DryRunProjectUpdater satisfies projectUpdater without ever calling
// entity-service — it only logs what would have been written. This is the
// entire mechanism behind DRY_RUN: main.go injects this instead of the real
// *entity.Client when dry-run is active, and processProject/Run are
// completely unaware of the distinction — they just call whichever
// projectUpdater they were given.
type DryRunProjectUpdater struct {
	Logger *slog.Logger
}

// UpdateProject logs the write that would have happened and always succeeds.
func (u *DryRunProjectUpdater) UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error) {
	u.Logger.InfoContext(ctx, "dry-run: would update project", "projectID", id, "body", string(body))
	return []byte(`{}`), nil
}
