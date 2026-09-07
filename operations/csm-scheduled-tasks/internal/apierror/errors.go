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

// Package apierror defines a typed error returned by upstream service
// clients when a non-2xx response is received — based on
// integrations/csm-notification-service's own internal/apierror, but
// deliberately diverges on one point: see Error's own doc comment.
package apierror

import "fmt"

// Error is returned when an upstream service responds with a non-2xx status.
type Error struct {
	StatusCode int
	// Body is kept on the struct for a caller that explicitly wants to
	// inspect it (e.g. via errors.As), but is deliberately excluded from
	// Error()'s own message — unlike
	// integrations/csm-notification-service's identical-looking type, whose
	// Error() does include it. A raw response body from entity-service or
	// the email-sending service can carry sensitive data, and Error()'s
	// return value flows into ordinary slog calls at every call site in
	// this component, including ones that end up in an alert email body —
	// see internal/engine.recordFailure. Body itself isn't removed, so
	// that path remains available to a caller that consciously chooses it.
	Body string
}

// Error returns a status-only message — see the Body field's own doc
// comment for why the response body is deliberately not included here.
func (e *Error) Error() string {
	return fmt.Sprintf("upstream returned %d", e.StatusCode)
}
