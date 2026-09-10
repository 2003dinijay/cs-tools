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

import "time"

// dateOnlyLayout is the wire format for a calendar date with no time component,
// matching the upstream DateString type the portal was built against.
const dateOnlyLayout = "2006-01-02"

// DateOnly renders a timestamp as a bare calendar date, or nil when there is no
// date to render.
//
// Calendar dates must not reach the portal as RFC3339 timestamps. Ballerina types
// these fields as DateString (constrained to ^\d{4}-\d{2}-\d{2}$) and the frontend
// treats the value as opaque text it can hand straight back as a filter — the
// time-tracking page defaults its startDate filter to project.startDate verbatim.
// Emitting "2026-06-16T00:00:00Z" there produced
// "startDate=2026-06-16T00:00:00Z", which the upstream rejects on its own date
// pattern while the locally-formatted endDate beside it was accepted.
//
// A zero time is treated as absent rather than rendered as "0001-01-01": these
// fields are nullable upstream, and a fabricated date is worse than a missing one.
func DateOnly(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := t.Format(dateOnlyLayout)
	return &s
}

// DateOnlyValue is DateOnly for a non-pointer timestamp, for fields entity-service
// models as always-present but which the portal still exposes as nullable —
// entity-service returns a zero time when the upstream omitted the date.
func DateOnlyValue(t time.Time) *string {
	return DateOnly(&t)
}
