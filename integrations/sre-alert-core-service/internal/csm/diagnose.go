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

package csm

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/url"
	"strings"

	"github.com/sony/gobreaker/v2"
	"golang.org/x/oauth2"
)

// Error kinds name where a call to csm-integration-service failed; http_status, response_read_failed and response_decode_failed reached it.
const (
	KindHTTPStatus     = "http_status"
	KindTokenFailed    = "token_request_failed"
	KindTimeout        = "timeout"
	KindDNS            = "dns_lookup_failed"
	KindConnection     = "connection_failed"
	KindTLS            = "tls_failed"
	KindNonHTTPS       = "non_https_endpoint"
	KindBreakerOpen    = "circuit_breaker_open"
	KindCallerCanceled = "caller_canceled"
	KindDecode         = "response_decode_failed"
	KindResponseRead   = "response_read_failed"
	KindOther          = "transport_error"
)

// ResponseReadError is a body that failed to read after csm-integration-service answered with StatusCode.
type ResponseReadError struct {
	StatusCode int
	Err        error
}

func (e *ResponseReadError) Error() string { return e.Err.Error() }

func (e *ResponseReadError) Unwrap() error { return e.Err }

// ErrorKind classifies err by where the call failed; "" for nil.
func ErrorKind(err error) string {
	if err == nil {
		return ""
	}
	var apiErr *Error
	var readErr *ResponseReadError
	var retrieveErr *oauth2.RetrieveError
	var dnsErr *net.DNSError
	var opErr *net.OpError
	var certErr *tls.CertificateVerificationError
	var unknownAuthErr x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var recordErr tls.RecordHeaderError
	switch {
	case errors.As(err, &apiErr):
		return KindHTTPStatus
	case errors.As(err, &readErr):
		return KindResponseRead
	case errors.Is(err, gobreaker.ErrOpenState), errors.Is(err, gobreaker.ErrTooManyRequests):
		return KindBreakerOpen
	case errors.Is(err, errCallerDone), errors.Is(err, context.Canceled):
		return KindCallerCanceled
	case errors.As(err, &retrieveErr), strings.Contains(err.Error(), "oauth2:"):
		return KindTokenFailed
	case errors.Is(err, context.DeadlineExceeded), isTimeout(err):
		return KindTimeout
	case errors.As(err, &dnsErr):
		return KindDNS
	case errors.As(err, &certErr), errors.As(err, &unknownAuthErr), errors.As(err, &hostnameErr), errors.As(err, &recordErr):
		return KindTLS
	case strings.Contains(err.Error(), "refusing non-HTTPS"):
		return KindNonHTTPS
	case errors.As(err, &opErr):
		return KindConnection
	case strings.Contains(err.Error(), "csm: decode"):
		return KindDecode
	default:
		return KindOther
	}
}

func isTimeout(err error) bool {
	var urlErr *url.Error
	return errors.As(err, &urlErr) && urlErr.Timeout()
}

// ErrorAttrs are the slog attributes that say whether a failed call reached csm-integration-service and what came back.
func ErrorAttrs(err error) []any {
	kind := ErrorKind(err)
	var readErr *ResponseReadError
	read := errors.As(err, &readErr)
	attrs := []any{"error_kind", kind, "reached_csm", kind == KindHTTPStatus || kind == KindDecode || read}
	if read {
		attrs = append(attrs, "status_code", readErr.StatusCode)
	}
	var apiErr *Error
	if errors.As(err, &apiErr) {
		attrs = append(attrs, "status_code", apiErr.StatusCode)
		if apiErr.Location != "" {
			attrs = append(attrs, "redirect_location", apiErr.Location)
		}
	}
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) && retrieveErr.Response != nil {
		attrs = append(attrs, "token_status_code", retrieveErr.Response.StatusCode)
	}
	return append(attrs, "error", err.Error())
}
