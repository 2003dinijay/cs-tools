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

package dto

import "strings"

// projectTypeDisplayLabels turns entity-service's subscription-type enum into the
// label the portal matches on.
//
// These strings are not cosmetic — they are compared for equality against the
// frontend's ProjectType enum (src/types/permission.ts) and drive real behaviour:
//
//   - Development Support locks case severity to S4
//   - Cloud Support and Cloud Evaluation Support auto-pick the primary product on
//     case and service-request creation
//   - anything other than Managed Cloud Subscription has S0 (Catastrophic)
//     filtered out of the severity options
//
// So a wrong label is worse than a missing one: it silently changes which
// severities a customer can pick. Keep them byte-identical to that enum.
//
// entity-service derives its enum mechanically from the ServiceNow display name
// (lowercase, spaces to underscores — see snTypeNameToSubscriptionType), but an
// explicit table is used here rather than reversing that transform, because
// title-casing would mangle any future value containing an acronym.
var projectTypeDisplayLabels = map[string]string{
	"managed_cloud_subscription": "Managed Cloud Subscription",
	"cloud_support":              "Cloud Support",
	"cloud_evaluation_support":   "Cloud Evaluation Support",
	"evaluation_subscription":    "Evaluation Subscription",
	"subscription":               "Subscription",
	"development_support":        "Development Support",
	"professional_services":      "Professional Services",
	// No frontend ProjectType entry exists for these two, so no behaviour keys
	// off them; they are mapped anyway so the field is never blank.
	"internal":                "Internal",
	"platformer_subscription": "Platformer Subscription",
}

// projectTypeRef builds the {id, label} reference the frontend reads as
// project.type.
//
// entity-service returns the subscription type as a plain enum string, while the
// frontend's ProjectListItem and ProjectDetails both declare `type: IdLabelRef`
// and read `type.label`. Sending only the enum under a different key left
// project.type undefined, which hid the Operations nav item outright
// (SideBar.tsx gates it on isProjectTypeResolved) and made every project-type
// rule above take its "not that type" branch.
//
// The id carries the enum value rather than a numeric choice-list id: no such id
// is available from entity-service, nothing in the portal reads project
// type.id (only deployment type.id is read), and a stable non-empty value beats
// inventing a fake number. An unrecognised enum passes through as its own label
// rather than blanking the field, the same tolerance the case *Ref helpers use.
func projectTypeRef(subscriptionType string) *IDLabelRef {
	key := strings.ToLower(strings.TrimSpace(subscriptionType))
	if key == "" {
		return nil
	}
	label, ok := projectTypeDisplayLabels[key]
	if !ok {
		label = subscriptionType
	}
	return &IDLabelRef{ID: key, Label: label}
}
