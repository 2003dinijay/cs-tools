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

// Package handler implements the HTTP layer: decoding requests, calling the
// entity-service client, mapping responses via the dto package, and writing
// JSON back to the customer-portal frontend.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
)

// maxRequestBodyBytes caps request bodies accepted by search/create/update endpoints.
const maxRequestBodyBytes = 1 << 20 // 1 MiB

// maxZipUploadBytes caps the raw (non-JSON) binary body accepted by
// POST /deployment-usages.
const maxZipUploadBytes = 25 << 20 // 25 MiB

// uuidRe validates path parameters that are expected to be UUIDs.
var uuidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// sysidRe matches a bare ServiceNow sysid: 32 hex characters, no dashes.
var sysidRe = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)

// isAttachmentID reports whether id is a usable attachment identifier — either a
// dashed UUID or a bare ServiceNow sysid.
//
// Attachment ids reach this API in both forms. The attachments list returns
// dashed UUIDs (entity-service applies sysidToUUID), but an inline comment image
// is referenced only by its `<img src="…/<sysid>.iix">`, and the frontend
// extracts that 32-hex sysid directly (see the .iix regex in
// features/support/utils/support.ts) before asking for its content. Rejecting
// the sysid form makes every inline comment image fail.
//
// The Ballerina backend accepts both because its path parameter is typed
// `entity:IdString`, a plain string alias with no format constraint. This keeps
// the same contract while still refusing anything that is neither shape, so the
// value remains safe to place in an upstream URL path.
func isAttachmentID(id string) bool {
	return uuidRe.MatchString(id) || sysidRe.MatchString(id)
}

// Error message constants matching the customer-portal error vocabulary.
const (
	ErrMsgUnauthorized = "You are not authorized to perform this action. Please try again."
	ErrMsgForbidden    = "Access to the requested resource is forbidden!"
	ErrMsgNotFound     = "The requested resource was not found!"
	ErrMsgBadRequest   = "Invalid request payload."
	ErrMsgTooLarge     = "Request body too large."
	ErrMsgInternal     = "An internal server error occurred. Please try again later."
	ErrMsgInvalidUUID  = "Invalid UUID format."
	errMsgReadBody     = "Failed to read request body."
)

// errorBody is the JSON error payload format matching the customer-portal pattern.
type errorBody struct {
	Message string `json:"message"`
}

// writeError writes a JSON error response: {"message": "..."}.
func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(errorBody{Message: message})
}

// writeJSONValue marshals v and writes the result as a JSON response.
func writeJSONValue(w http.ResponseWriter, statusCode int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(data) // #nosec G705 -- Content-Type: application/json already set; SecurityHeaders middleware adds X-Content-Type-Options: nosniff
}

// mapUpstreamError translates an entity-service error to an HTTP response
// using this backend's own status-code mapping.
//
// For 400, apiErr.Body (entity-service's own validation message — e.g.
// "caseTypes must be valid UUIDs", "Reason is required when escalating a
// case.") is returned verbatim instead of a generic fallback: entity-service
// constructs these specifically to be safe and useful to show the caller
// (see its apierror.ValidationError), and swallowing the specific reason
// into "Invalid request payload." for every 400 makes debugging a rejected
// request needlessly harder for API consumers. ErrMsgBadRequest is only used
// when entity-service didn't supply a body at all.
func mapUpstreamError(w http.ResponseWriter, err error, fallbackMsg string) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		case http.StatusForbidden:
			writeError(w, http.StatusForbidden, ErrMsgForbidden)
		case http.StatusNotFound:
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
		case http.StatusBadRequest:
			msg := apiErr.Body
			if msg == "" {
				msg = ErrMsgBadRequest
			}
			writeError(w, http.StatusBadRequest, msg)
		case http.StatusConflict, http.StatusUnprocessableEntity:
			writeError(w, apiErr.StatusCode, apiErr.Body)
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			writeError(w, http.StatusServiceUnavailable, fallbackMsg)
		default:
			writeError(w, http.StatusInternalServerError, fallbackMsg)
		}
		return
	}
	writeError(w, http.StatusInternalServerError, fallbackMsg)
}

// summarizeErr returns a log-safe description of err: for a typed
// *apierror.Error, the upstream status code and, if present, its Body
// (entity-service's own error message, extracted from its
// {"code","message"} response shape by entity.newUpstreamError — Body is
// left empty rather than falling back to a raw response excerpt, so this
// never logs unbounded or non-message upstream content) — or a fixed
// generic message otherwise. An unrecognized error can come from the
// underlying HTTP client (e.g. a net/url.Error), which stringifies with the
// full request URL including query parameters, so its raw text is not safe
// to log verbatim.
func summarizeErr(err error) string {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		if apiErr.Body == "" {
			return fmt.Sprintf("upstream status %d", apiErr.StatusCode)
		}
		return fmt.Sprintf("upstream status %d: %s", apiErr.StatusCode, apiErr.Body)
	}
	// Below here the failure carries no upstream HTTP status: a malformed
	// upstream URL, a transport or TLS failure, a token fetch that never got a
	// response, a cancelled context, or a response body this backend could not
	// decode. All of them used to collapse into one opaque "upstream request
	// failed", which made a 500 from any of these causes indistinguishable in
	// the logs — a recommendations 500 took several rounds of guesswork to place
	// because the log could not say whether the URL, the credentials, or the
	// response shape was at fault.
	//
	// Categories and schema facts only. In particular this never logs
	// err.Error() for a *url.Error, because that appends the full request URL
	// (which for other clients can carry filter query params); url.Error.Err on
	// its own does not.
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "upstream request timed out"
	case errors.Is(err, context.Canceled):
		return "upstream request canceled"
	}

	// Field/Type/Value here describe the contract, not the payload — e.g.
	// `field "createdBy" expects string, got object` — so they are safe to log
	// and they name the exact mismatch, which is the one thing that makes a
	// Ballerina-to-Go field/type drift immediately obvious.
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return fmt.Sprintf("response decode failed: field %q expects %s, got %s", typeErr.Field, typeErr.Type, typeErr.Value)
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return fmt.Sprintf("response decode failed: malformed JSON at byte %d", syntaxErr.Offset)
	}

	// Op distinguishes a URL that would not parse ("parse") from a request that
	// could not be sent ("Post"/"Get"); Err carries the reason without the URL —
	// "unsupported protocol scheme", "invalid control character in URL", a dial
	// failure — which is exactly what separates a misconfigured base URL from an
	// unreachable or unauthorized upstream.
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return fmt.Sprintf("upstream %s failed: %s", urlErr.Op, urlErr.Err)
	}

	return "upstream request failed"
}

// readJSONBody caps r.Body at maxRequestBodyBytes, reads it fully, and
// validates it is well-formed JSON. Writes the appropriate error response and
// returns ok=false if the body is too large, unreadable, or invalid JSON.
func readJSONBody(w http.ResponseWriter, r *http.Request) (body []byte, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, isTooLarge := err.(*http.MaxBytesError); isTooLarge {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, false
	}
	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, false
	}
	return body, true
}

// readBinaryBody caps r.Body at maxBytes and reads it fully, for endpoints
// that accept a raw (non-JSON) request body. Writes the appropriate error
// response and returns ok=false if the body is too large or unreadable.
func readBinaryBody(w http.ResponseWriter, r *http.Request, maxBytes int64) (body []byte, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if _, isTooLarge := err.(*http.MaxBytesError); isTooLarge {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, false
	}
	return body, true
}

// conversationStateConverted is the conversation state set when a case has been
// created from a chat. It outranks RESOLVED — see handleMessage's resolved
// branch in websocket.go, which refuses to downgrade it.
const conversationStateConverted = "CONVERTED"
