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

package handler

import "github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"

const (
	caseStateOpen             = "open"
	caseStateWorkInProgress   = "work_in_progress"
	caseStateWaitingOnWSO2    = "waiting_on_wso2"
	caseStateAwaitingInfo     = "awaiting_info"
	caseStateSolutionProposed = "solution_proposed"
	caseStateClosed           = "closed"
	caseStateReopened         = "reopened"
)

// customerNextStates is this portal's own copy of the legacy ServiceNow
// scoped-app's ALLOWED_STATE_TRANSITIONS table -- deliberately narrower than
// apps/csm-portal/backend's nextStates (that one ports SUPERUSER_STATE_TRANSITIONS,
// the agent-facing table). A customer's own case-update action is almost
// always "close it"; the three states that can route back through WSO2
// (awaiting_info, solution_proposed) also let the customer send it back to
// waiting_on_wso2, and solution_proposed is the one state a customer can
// reopen from, mirroring the legacy table's own asymmetry. This backend had
// no transition guard at all before this -- PatchCase forwarded any stateKey
// a customer sent straight to entity-service, which enforces no from->to
// graph of its own (see entity-service's own CLAUDE.md: it only checks that
// the target is a real state, never that the move is legal).
func customerNextStates(state string) []string {
	switch state {
	case caseStateOpen:
		return []string{caseStateClosed}
	case caseStateWorkInProgress:
		return []string{caseStateClosed}
	case caseStateWaitingOnWSO2:
		return []string{caseStateClosed}
	case caseStateSolutionProposed:
		return []string{caseStateClosed, caseStateReopened, caseStateWaitingOnWSO2}
	case caseStateAwaitingInfo:
		return []string{caseStateClosed, caseStateWaitingOnWSO2}
	case caseStateReopened:
		return []string{caseStateClosed}
	default: // unknown/closed: terminal
		return []string{}
	}
}

// isValidCustomerStateTransition reports whether a customer may move a case
// from its current state to the requested one, per customerNextStates. Both
// arguments are normalized via dto.NormalizedCaseStateEnum first, so a raw
// ServiceNow display label ("Work In Progress") and entity-service's own
// domain enum ("work_in_progress") resolve the same way; an unrecognized
// current state normalizes to "", which customerNextStates treats as
// terminal -- a customer can never transition a case whose current state
// this portal doesn't recognize.
func isValidCustomerStateTransition(from, to string) bool {
	normalizedFrom := dto.NormalizedCaseStateEnum(from)
	normalizedTo := dto.NormalizedCaseStateEnum(to)
	if normalizedTo == "" {
		return false
	}
	for _, s := range customerNextStates(normalizedFrom) {
		if s == normalizedTo {
			return true
		}
	}
	return false
}
