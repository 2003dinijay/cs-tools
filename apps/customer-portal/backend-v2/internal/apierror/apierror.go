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

// Package apierror defines the error type used to carry upstream (entity-service)
// HTTP failures back through the client layer to the handler layer.
package apierror

import (
	"encoding/json"
	"fmt"
)

// Error wraps a non-2xx response from an upstream service call.
type Error struct {
	StatusCode int
	Body       string
}

func (e *Error) Error() string {
	return fmt.Sprintf("upstream returned %d: %s", e.StatusCode, e.Body)
}

// upstreamErrorBody is the {"message": "..."} shape every upstream service
// this backend calls uses for its own error responses (entity-service's is a
// superset, {"code":...,"message":"..."}, which unmarshals the same way).
type upstreamErrorBody struct {
	Message string `json:"message"`
}

// NewUpstreamError builds an *Error from a non-2xx upstream HTTP response.
// Body is set to the upstream's own "message" field when the response is the
// expected {"message": "..."} shape, and left empty otherwise. Every upstream
// client in this backend must construct its errors through this function
// rather than falling back to a raw response excerpt: callers already treat
// an empty Body as "no specific message available" (both mapUpstreamError's
// 400 case and writeUpstreamMessage fall back to a fixed message), so a raw
// excerpt is never necessary — and logging or returning one to the frontend
// risks leaking unbounded, non-message upstream content (e.g. a gateway HTML
// error page).
func NewUpstreamError(statusCode int, rawBody []byte) *Error {
	var body upstreamErrorBody
	if err := json.Unmarshal(rawBody, &body); err == nil && body.Message != "" {
		return &Error{StatusCode: statusCode, Body: body.Message}
	}
	return &Error{StatusCode: statusCode}
}
