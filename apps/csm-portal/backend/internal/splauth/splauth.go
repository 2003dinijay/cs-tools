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

// Package splauth implements SupportPortalLite's group-membership checks.
// The Ballerina backend being ported enforces five independent,
// configurable Asgardeo group lists (verified by reading
// modules/authJWT/auth.bal and modules/operations/operations.bal):
//
//   - allowedGroups — a blanket gate applied to every request
//     (authJWT.imposeGlobalRules), ported as the first check in every
//     /spl/* handler via the handler package's requireSPLGroups helper.
//   - addWorknoteGroups, addEscalationGroups, downloadAttachmentGroups,
//     usageMetricsGroups — additional, narrower checks layered on top of
//     the blanket gate for specific endpoints (operations.bal's
//     addWorknote/createCaseEscalation/downloadAttachment/
//     checkUsageMetricsAccess functions).
//
// IsAuthorized is the single primitive shared by all five checks; each
// endpoint calls it with whichever configured list applies.
package splauth

// IsAuthorized reports whether userGroups contains at least one group from
// allowedGroups. An empty/nil allowedGroups authorizes nobody, matching the
// Ballerina isUserAuthorized behavior (an unconfigured allow-list denies
// every caller rather than defaulting open).
func IsAuthorized(userGroups, allowedGroups []string) bool {
	for _, allowed := range allowedGroups {
		for _, got := range userGroups {
			if got == allowed {
				return true
			}
		}
	}
	return false
}
