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

package domain

import "time"

// Runbook is one entry of the alert-name to runbook catalog (migration 0241).
type Runbook struct {
	ID        string `json:"id"`
	AlertName string `json:"alertName"`
	// RunbookURL is nil when the alert is known to have no runbook yet.
	RunbookURL *string   `json:"runbookUrl"`
	CreatedOn  time.Time `json:"createdOn"`
	CreatedBy  string    `json:"createdBy"`
	UpdatedOn  time.Time `json:"updatedOn"`
	UpdatedBy  string    `json:"updatedBy"`
}

// RunbookList is the body of GET /runbooks.
type RunbookList struct {
	Runbooks []Runbook `json:"runbooks"`
}

// SaveRunbookRequest is the body of POST /runbooks and PUT /runbooks/{id}.
type SaveRunbookRequest struct {
	AlertName  string  `json:"alertName"`
	RunbookURL *string `json:"runbookUrl"`
}

// ImportRunbooksRequest is the body of POST /runbooks/import: each entry is created, or updated when its alert name exists.
type ImportRunbooksRequest struct {
	Runbooks []SaveRunbookRequest `json:"runbooks"`
}

// ImportRunbooksResponse counts what an import did.
type ImportRunbooksResponse struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
}
