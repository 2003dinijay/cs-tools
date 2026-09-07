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

import "strconv"

// deploymentTypeIDs mirrors entity-service's private deploymentTypeToKey
// table (internal/service/sn_deployment_service.go) — the ServiceNow
// choice-list integer key for each DeploymentType enum value. The frontend
// was built against the old Ballerina backend, which forwarded this raw
// numeric key (deploymentTypeKey/typeKey) directly; entity-service's own
// contract is the plain string enum, so this backend translates both ways,
// same pattern as case_enum_mapping.go.
var deploymentTypeIDs = map[string]int{
	"development":        1,
	"qa":                 2,
	"staging":            3,
	"stress":             4,
	"uat":                5,
	"primary_production": 6,
}

var deploymentTypeIDToEnum = func() map[int]string {
	m := make(map[int]string, len(deploymentTypeIDs))
	for enum, id := range deploymentTypeIDs {
		m[id] = enum
	}
	return m
}()

// deploymentTypeLabels are portal-owned display labels for each deployment
// type enum value.
var deploymentTypeLabels = map[string]string{
	"development":        "Development",
	"qa":                 "QA",
	"staging":            "Staging",
	"stress":             "Stress",
	"uat":                "UAT",
	"primary_production": "Primary Production",
}

// deploymentTypeRef converts entity-service's string enum into an
// {id, label} pair, id being the ServiceNow numeric key as a string.
func deploymentTypeRef(enum string) *IDLabelRef {
	if enum == "" {
		return nil
	}
	label := deploymentTypeLabels[enum]
	if label == "" {
		label = enum
	}
	id := ""
	if key, ok := deploymentTypeIDs[enum]; ok {
		id = strconv.Itoa(key)
	}
	return &IDLabelRef{ID: id, Label: label}
}

// deploymentTypeIDToEnumPtr translates a frontend-supplied numeric
// deployment-type key into entity-service's string enum, returning nil for
// an unrecognized id (never forwarded to entity-service as a raw number).
func deploymentTypeIDToEnumPtr(key int) *string {
	enum, ok := deploymentTypeIDToEnum[key]
	if !ok {
		return nil
	}
	return &enum
}
