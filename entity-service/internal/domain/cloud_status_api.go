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

// The public cloud status dashboard's read API.
//
// These types are the Postgres-backed replacement for five ServiceNow Scripted
// REST APIs that wso2-enterprise/uptime-dashboard calls today. Their shapes are
// NOT ours to choose: the dashboard's frontend already renders them, so the
// JSON here reproduces what those scripts emit, field name for field name.
// Where a name reads oddly for Go (display_name, subgroups) that is why.
//
// The scripts live in that repo under servicenow/scripted-rest-api/ and were
// read directly rather than inferred from responses.

// CloudStatusMonitorGroup is one named group within a region, e.g. "Login" or
// "Development", holding the individual monitors shown beneath it.
type CloudStatusMonitorGroup struct {
	DisplayName string                       `json:"display_name"`
	Subgroups   []CloudStatusMonitorSubgroup `json:"subgroups"`
}

// CloudStatusMonitorSubgroup is a single monitored component as the dashboard
// renders it: a row with a status light, an uptime figure and, during an
// incident, a message.
type CloudStatusMonitorSubgroup struct {
	// Description is a POINTER so an absent one serialises as null, not "".
	// ServiceNow emits null, and the frontend keys a tooltip icon off this
	// field -- sending an empty string where it has always received null is
	// the kind of difference that renders.
	Description *string `json:"description"`
	DisplayName string  `json:"display_name"`

	// Status is the dashboard's integer scale, not the Postgres enum:
	// 0 Operational · 1 Maintenance · 2 Degraded · 3 Outage. The mapping is
	// fixed by the frontend's own constants.js and by the flow that writes it.
	Status int `json:"status"`

	Availability []CloudStatusAvailability `json:"availability"`

	// Message is the short description of an ONGOING outage affecting this
	// component, and only when that outage's type agrees with the status the
	// monitor currently shows. Empty otherwise -- see the repository for why
	// that agreement check is not redundant.
	Message string `json:"message"`
}

// CloudStatusAvailability is one uptime figure for one window.
//
// Availability is a STRING, not a number, because the ServiceNow script emits
// it as one and the frontend renders it verbatim. Sending 100 where the
// dashboard has always received "100" is the kind of difference that survives
// every test and changes what a customer sees.
type CloudStatusAvailability struct {
	Availability string `json:"availability"`
	Duration     string `json:"duration"`
}

// CloudStatusIncident is one past or ongoing outage as the incident history
// renders it.
type CloudStatusIncident struct {
	ID               string `json:"id"`
	Begin            string `json:"begin"`
	End              string `json:"end"`
	Type             string `json:"type"`
	Status           string `json:"status"`
	ShortDescription string `json:"short_description"`

	// Expanded drives a UI accordion and is always false on the wire. Kept
	// because the frontend reads it; dropping it would be a frontend change.
	Expanded bool `json:"expanded"`
}

// CloudStatusIncidentMonth is one calendar month's incidents.
type CloudStatusIncidentMonth struct {
	Incidents []CloudStatusIncident `json:"incidents"`
}

// CloudStatusMonitorsResponse is keyed by region -- "cp - us", "eu - dp - eu"
// and so on -- exactly as the dashboard groups its columns.
type CloudStatusMonitorsResponse map[string][]CloudStatusMonitorGroup

// CloudStatusIncidentsResponse is keyed "YYYY-M", e.g. "2026-9". Not
// zero-padded, because the script builds the key by string concatenation and
// the frontend matches on it.
type CloudStatusIncidentsResponse map[string]CloudStatusIncidentMonth

// cloudStatusDashboardStatus maps the stored enum to the dashboard's integer.
//
// MAJOR_OUTAGE has no integer here on purpose: the dashboard's own constants
// define 0-3 only, and the flow that writes these values never produces 4. A
// monitor somehow holding MAJOR_OUTAGE degrades to 3, the most severe the
// dashboard can render, rather than to 0.
var cloudStatusDashboardStatus = map[CloudMonitorStatus]int{
	CloudMonitorStatusOperational:   0,
	CloudMonitorStatusMaintenance:   1,
	CloudMonitorStatusDegraded:      2,
	CloudMonitorStatusPartialOutage: 3,
	CloudMonitorStatusMajorOutage:   3,
}

// DashboardStatus converts a stored monitor status to the dashboard's integer.
// An unknown value becomes 0, matching the script's behaviour on an empty
// u_status.
func DashboardStatus(s CloudMonitorStatus) int {
	return cloudStatusDashboardStatus[s]
}

// outageTypeForStatus is the inverse agreement check the monitors script
// performs before attaching an outage's message to a component.
//
// The script only shows the message when the ongoing outage's TYPE matches the
// status the monitor is currently displaying: planned with 1, degradation with
// 2, outage with 3. That looks redundant -- the flow sets the status from the
// type in the first place -- and is not: the two are written by different
// paths at different times, so a monitor can be showing Degraded while the
// outage that caused it has since been retyped. The check suppresses a message
// that would contradict the light next to it.
var outageTypeForStatus = map[int]string{
	1: "PLANNED",
	2: "DEGRADATION",
	3: "OUTAGE",
}

// MessageAgreesWithStatus reports whether an ongoing outage's type matches the
// status a monitor is showing.
func MessageAgreesWithStatus(status int, outageType string) bool {
	want, ok := outageTypeForStatus[status]
	return ok && want == outageType
}
