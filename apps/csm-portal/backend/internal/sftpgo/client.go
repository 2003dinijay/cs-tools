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

// Package sftpgo is an HTTP client for the small subset of SFTPGo's REST API
// this backend calls when the SFTPGo-backed attachment-storage feature flag
// (SFTPGO_ATTACHMENT_STORAGE_ENABLED) is on: minting a short-lived per-user
// access token (used only server-side, to authenticate this backend's own
// calls into SFTPGo's REST API — never handed to the browser), and creating
// short-lived shares scoped to a single storage path, either read-only
// (public download/inline-image shares) or write-only (upload shares). This
// package never touches attachment bytes: uploads and downloads always go
// directly between the browser and SFTPGo, authenticated with nothing more
// than the share id a Share.Scope-limited share carries — never a bearer
// token — see internal/handler.AttachmentStorageHandler for the call sites.
package sftpgo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// maxErrBodyBytes bounds how much of a non-2xx response body is retained on
// an *apierror.Error, mirroring internal/entity's CustomerEntityClient.
const maxErrBodyBytes = 256

// Config holds the configuration for a SFTPGo Client.
type Config struct {
	// BaseURL is SFTPGo's REST API base (used for both the token-mint and
	// share-creation calls below), and is also the host the FE is told to
	// call directly for the chunked/TUS upload once it has a minted token —
	// SFTPGo serves both its REST API and its upload endpoints from the same
	// httpd listener, so one base URL covers both.
	BaseURL string
	// PublicBaseURL is the host used to construct the public share URL
	// returned by CreateShare's caller (see PublicShareURL). SFTPGo commonly
	// fronts its WebClient (share pages) on a different public host/port
	// than its REST API, so this is a separate, optional value; when unset
	// it defaults to BaseURL.
	PublicBaseURL string
}

// Client is a minimal SFTPGo REST API client scoped to token-mint and
// share-creation — the only two calls this backend ever makes to SFTPGo.
type Client struct {
	http          *http.Client
	baseURL       string
	publicBaseURL string
}

// NewClient constructs a SFTPGo Client from cfg.
func NewClient(cfg Config) *Client {
	publicBaseURL := cfg.PublicBaseURL
	if publicBaseURL == "" {
		publicBaseURL = cfg.BaseURL
	}
	return &Client{
		http: &http.Client{
			Timeout:       15 * time.Second,
			CheckRedirect: refuseInsecureRedirect,
		},
		baseURL:       strings.TrimRight(cfg.BaseURL, "/"),
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}
}

// refuseInsecureRedirect is an http.Client.CheckRedirect override that
// refuses to follow any redirect whose target is not HTTPS. Go's default
// CheckRedirect copies the Authorization header onto a same-host redirect,
// which would silently leak this client's bearer token (see MintToken,
// CreateShare) over cleartext on an HTTPS-to-HTTP downgrade redirect —
// something a compromised or misconfigured SFTPGo instance, or a
// man-in-the-middle, could trigger without this check.
func refuseInsecureRedirect(req *http.Request, via []*http.Request) error {
	if req.URL.Scheme != "https" {
		return fmt.Errorf("sftpgo: refusing to follow redirect to non-https URL %q", req.URL.Redacted())
	}
	return nil
}

// BaseURL returns the configured REST API base URL, verbatim — handed back
// to the FE by AttachmentStorageHandler.MintUploadToken as the host it
// should call directly for the upload itself.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Token is the response body of SFTPGo's GET /api/v2/user/token. ExpiresAt is
// kept as raw JSON and passed through unmodified rather than parsed into a
// Go time value, since its exact wire type (string vs. epoch number) was not
// verified against a live instance for this change.
type Token struct {
	AccessToken string          `json:"access_token"`
	ExpiresAt   json.RawMessage `json:"expires_at"`
}

// MintToken calls SFTPGo's GET /api/v2/user/token using HTTP Basic auth:
// username is the caller's email claim, password is the caller's raw
// gateway-issued JWT (the x-jwt-assertion header value this backend itself
// already validated). SFTPGo's external_auth_hook (see
// integrations/sftpgo-authentication-service's /external-auth-hook)
// independently re-validates that JWT against the same JWKS/issuer/audience
// this backend trusts, so the "password" here is never a SFTPGo-native
// credential — it is the same bearer token the caller already presented to
// this backend.
func (c *Client) MintToken(ctx context.Context, email, jwtAssertion string) (*Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/user/token", nil)
	if err != nil {
		return nil, fmt.Errorf("sftpgo: build token request: %w", err)
	}
	req.SetBasicAuth(email, jwtAssertion)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sftpgo: token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("sftpgo: read token response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: truncate(body)}
	}

	var tok Token
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("sftpgo: decode token response: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("sftpgo: token response carried no access_token")
	}
	return &tok, nil
}

// ShareScopeRead and ShareScopeWrite are SFTPGo's Share.Scope values for a
// read-only (download) share and a write-only (upload) share, respectively.
// Verified against SFTPGo's own OpenAPI spec for this change (unlike several
// other SFTPGo API shapes elsewhere in this file, which remain unverified
// assumptions — see their own doc comments).
const (
	ShareScopeRead  = 1
	ShareScopeWrite = 2
)

// shareCreateRequest is the request body of POST /api/v2/user/shares.
type shareCreateRequest struct {
	Paths     []string `json:"paths"`
	Scope     int      `json:"scope"`
	ExpiresAt int64    `json:"expires_at"`
}

// shareCreateResponseBody is the fallback shape checked when the share id is
// not present on the X-Object-Id response header (see CreateShare). Both
// field names are checked since which one (if either) SFTPGo actually uses
// here was not verified against a live instance.
type shareCreateResponseBody struct {
	ID      string `json:"id"`
	ShareID string `json:"share_id"`
}

// CreateShare calls SFTPGo's POST /api/v2/user/shares, authenticated as the
// caller via accessToken (minted by MintToken), to create a short-lived
// share for a single storage path with the given scope (ShareScopeRead or
// ShareScopeWrite). No password is ever set on the created share: for the
// read-only download-share path (AttachmentStorageHandler.CreateAttachmentShare)
// the share URL itself is the only credential handed out, and for the
// write-only upload-share path (AttachmentStorageHandler.MintUploadToken) the
// share id is only ever used server-to-server-adjacent, ambient in the TUS
// Upload-Metadata the browser sends — never paired with a bearer token or
// password. ttl controls how soon the share expires; callers should keep
// this short since a share is created fresh on every request that needs one
// (see CreateAttachmentShare and MintUploadToken — this is always a lazy,
// per-request operation, never an eager batch one). Returns the created
// share's id.
func (c *Client) CreateShare(ctx context.Context, accessToken, storageKey string, scope int, ttl time.Duration) (string, error) {
	reqBody, err := json.Marshal(shareCreateRequest{
		Paths:     []string{storageKey},
		Scope:     scope,
		ExpiresAt: time.Now().Add(ttl).UnixMilli(),
	})
	if err != nil {
		return "", fmt.Errorf("sftpgo: encode share request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/user/shares", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("sftpgo: build share request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("sftpgo: share request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("sftpgo: read share response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", &apierror.Error{StatusCode: resp.StatusCode, Body: truncate(body)}
	}

	// SFTPGo has historically returned the created object's id via the
	// X-Object-Id response header rather than the JSON body — confirmed
	// empirically against a real instance in a prior session. The JSON body
	// is only a fallback here and has NOT been independently re-verified for
	// this change.
	if id := resp.Header.Get("X-Object-Id"); id != "" {
		return id, nil
	}

	var decoded shareCreateResponseBody
	if err := json.Unmarshal(body, &decoded); err == nil {
		if decoded.ID != "" {
			return decoded.ID, nil
		}
		if decoded.ShareID != "" {
			return decoded.ShareID, nil
		}
	}

	return "", fmt.Errorf("sftpgo: share response carried no id (checked X-Object-Id header and id/share_id body fields)")
}

// PublicShareURL builds the public download URL for a share id.
//
// This is deliberately NOT "{publicBaseURL}/shares/{id}", despite that being
// what SFTPGo's own OpenAPI path naming might suggest — the working path,
// confirmed empirically against a real instance in a prior session, is
// "/web/client/pubshares/{id}".
func (c *Client) PublicShareURL(shareID string) string {
	return c.publicBaseURL + "/web/client/pubshares/" + url.PathEscape(shareID) + "?compress=false"
}

// tusResumableVersion is the TUS protocol version this client and the
// frontend's uploadFileViaTus both advertise via the Tus-Resumable header.
const tusResumableVersion = "1.0.0"

// UploadBytes writes data directly to SFTPGo's share-authenticated
// chunked/TUS upload endpoint (POST /api/v2/shares-chunked-uploads, then a
// single PATCH carrying the whole payload at offset 0), driven server-side
// by this Go HTTP client rather than a browser.
//
// This mirrors, call for call, what the frontend's uploadFileViaTus does
// against the same endpoint (see
// apps/csm-portal/webapp/src/features/csm-cases/api/attachmentStorageTus.ts):
// same Upload-Metadata keys (path/share_id/mkdir_parents), same
// Tus-Resumable/Upload-Length/Upload-Offset headers, same
// application/offset+octet-stream PATCH body. It exists because the
// browser-driven two-phase flow (MintUploadToken + confirm) assumes the
// caller does not yet have the file's bytes; the inline-image extraction
// path (see internal/handler.InlineImageProcessor) has the bytes
// synchronously in-process already decoded from a data: URI, so it drives
// the same TUS mechanics itself rather than round-tripping through a
// browser that was never involved.
//
// shareID must be a write-scoped share (see CreateShare with
// ShareScopeWrite) whose root covers storageKey's parent directory — it is
// the entire upload credential, exactly as for the browser path; no bearer
// token is sent alongside it. storageKey's final path segment (after the
// last "/") is sent as the TUS "path" metadata, matching the frontend's
// convention of sending only the filename since the share's own root
// already covers the directory portion.
func (c *Client) UploadBytes(ctx context.Context, shareID, storageKey string, data []byte, contentType string) error {
	fileName := storageKey
	if idx := strings.LastIndex(storageKey, "/"); idx != -1 {
		fileName = storageKey[idx+1:]
	}
	uploadMetadata := strings.Join([]string{
		"path " + base64.StdEncoding.EncodeToString([]byte(fileName)),
		"share_id " + base64.StdEncoding.EncodeToString([]byte(shareID)),
		"mkdir_parents " + base64.StdEncoding.EncodeToString([]byte("true")),
	}, ",")

	createEndpoint := c.baseURL + "/api/v2/shares-chunked-uploads"
	createReq, err := http.NewRequestWithContext(ctx, http.MethodPost, createEndpoint, nil)
	if err != nil {
		return fmt.Errorf("sftpgo: build chunked-upload create request: %w", err)
	}
	// No Authorization header: the share id embedded in Upload-Metadata above
	// is the entire credential for this endpoint, exactly as for the
	// browser-driven upload — see this method's doc comment.
	createReq.Header.Set("Tus-Resumable", tusResumableVersion)
	createReq.Header.Set("Upload-Length", strconv.Itoa(len(data)))
	createReq.Header.Set("Upload-Metadata", uploadMetadata)

	createResp, err := c.http.Do(createReq)
	if err != nil {
		return fmt.Errorf("sftpgo: chunked-upload create request: %w", err)
	}
	createBody, err := io.ReadAll(createResp.Body)
	createResp.Body.Close()
	if err != nil {
		return fmt.Errorf("sftpgo: read chunked-upload create response: %w", err)
	}
	if createResp.StatusCode < http.StatusOK || createResp.StatusCode >= http.StatusMultipleChoices {
		return &apierror.Error{StatusCode: createResp.StatusCode, Body: truncate(createBody)}
	}

	// The TUS spec returns the upload's URL via Location, which may be
	// relative to the create endpoint's origin or an absolute URL — resolve
	// it the same way the frontend does, and refuse to PATCH anywhere outside
	// this client's own configured origin (a misconfigured or compromised
	// SFTPGo instance redirecting the upload elsewhere is exactly the risk
	// the frontend's uploadFileViaTus guards against with the same check).
	uploadURL := createEndpoint
	if location := createResp.Header.Get("Location"); location != "" {
		base, err := url.Parse(createEndpoint)
		if err != nil {
			return fmt.Errorf("sftpgo: parse chunked-upload create endpoint: %w", err)
		}
		resolved, err := url.Parse(location)
		if err != nil {
			return fmt.Errorf("sftpgo: parse chunked-upload Location header %q: %w", location, err)
		}
		resolvedURL := base.ResolveReference(resolved)
		if resolvedURL.Scheme != base.Scheme || resolvedURL.Host != base.Host {
			return fmt.Errorf("sftpgo: refusing to upload to an untrusted origin returned by the Location header: %q", resolvedURL.Redacted())
		}
		uploadURL = resolvedURL.String()
	}

	patchReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, uploadURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("sftpgo: build chunked-upload PATCH request: %w", err)
	}
	patchReq.Header.Set("Tus-Resumable", tusResumableVersion)
	patchReq.Header.Set("Upload-Offset", "0")
	patchReq.Header.Set("Content-Type", "application/offset+octet-stream")
	patchReq.ContentLength = int64(len(data))

	patchResp, err := c.http.Do(patchReq)
	if err != nil {
		return fmt.Errorf("sftpgo: chunked-upload PATCH request: %w", err)
	}
	patchBody, err := io.ReadAll(patchResp.Body)
	patchResp.Body.Close()
	if err != nil {
		return fmt.Errorf("sftpgo: read chunked-upload PATCH response: %w", err)
	}
	if patchResp.StatusCode < http.StatusOK || patchResp.StatusCode >= http.StatusMultipleChoices {
		return &apierror.Error{StatusCode: patchResp.StatusCode, Body: truncate(patchBody)}
	}
	return nil
}

// truncate bounds body to maxErrBodyBytes for inclusion on an *apierror.Error.
func truncate(body []byte) string {
	if len(body) > maxErrBodyBytes {
		body = body[:maxErrBodyBytes]
	}
	return string(body)
}
