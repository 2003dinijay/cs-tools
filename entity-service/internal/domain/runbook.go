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

// Runbook is one runbook of the catalog (migrations 0241-0243): a numbered link and the alert names that lead to it.
type Runbook struct {
	ID string `json:"id"`
	// Number is the runbook's own number (RB0000001), given when it is created and never changed.
	Number string `json:"number"`
	// RunbookURL is nil when the runbook is not written yet.
	RunbookURL *string `json:"runbookUrl"`
	// Alerts are the alert names an incident subject is matched against, ordered by name.
	Alerts    []string  `json:"alerts"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
	UpdatedOn time.Time `json:"updatedOn"`
	UpdatedBy string    `json:"updatedBy"`
}

// RunbookList is the body of GET /runbooks.
type RunbookList struct {
	Runbooks []Runbook `json:"runbooks"`
}

// SaveRunbookRequest is the body of POST /runbooks and PUT /runbooks/{id}.
type SaveRunbookRequest struct {
	RunbookURL *string  `json:"runbookUrl"`
	Alerts     []string `json:"alerts"`
}

// ImportRunbooksRequest is the body of POST /runbooks/import: each entry replaces the runbook that holds its first listed alert, or is created.
type ImportRunbooksRequest struct {
	Runbooks []SaveRunbookRequest `json:"runbooks"`
}

// MergeRunbooksRequest is the body of POST /runbooks/merge: the first runbook keeps its id and number and takes every alert of
// the others, which are removed; it is relinked as given.
type MergeRunbooksRequest struct {
	RunbookIDs []string `json:"runbookIds"`
	RunbookURL *string  `json:"runbookUrl"`
}

// UnmergeRunbookAlertsRequest is the body of POST /runbooks/{id}/unmerge: each alert leaves the runbook and becomes a
// runbook of its own, with a new number and the same link.
type UnmergeRunbookAlertsRequest struct {
	Alerts []string `json:"alerts"`
}

// ImportRunbooksResponse counts what an import did.
type ImportRunbooksResponse struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	// Removed counts the runbooks left without alerts because the file moved all of them elsewhere.
	Removed int `json:"removed"`
}
