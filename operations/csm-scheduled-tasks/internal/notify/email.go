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

// Package notify sends report/alert emails directly against the same
// internal email notification service integrations/csm-notification-service
// uses (POST /send-email, OAuth2 client credentials) — this component has
// no event-emission hook yet (see this component's own CLAUDE.md, "Future:
// events"), so it sends its own email rather than publishing an event for
// csm-notification-service to relay.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// emailTokenFetchTimeout is the HTTP client timeout for token-endpoint
// requests. Overridden in tests to keep them fast.
var emailTokenFetchTimeout = 10 * time.Second

// Config holds the configuration for the email client below.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	// FromAddress is the fixed "From" address used for every outgoing
	// email — a config value, not a SendEmail argument, so every email
	// this component sends comes from one pre-approved sender.
	FromAddress string
	// DebugRecipients, when set, turns on debug mail mode: every email
	// this client sends goes to these addresses instead of its intended
	// To/Cc, with "[Test copy]" prefixed to the subject and a notice at
	// the top of the body naming who it would have gone to. It is how a
	// report is exercised in a production deployment, real data and all,
	// without reaching its real audience. Empty is the normal mode.
	DebugRecipients []string
}

// Client is an HTTP client for the internal email notification service,
// authenticated via the OAuth2 client credentials grant. Tokens are
// acquired and refreshed automatically.
//
// NewClient never contacts the token endpoint itself, so a valid-looking
// but wrong URL only surfaces as an error the first time SendEmail is
// called — but it does now validate cfg.TokenURL/cfg.BaseURL are https
// (loopback http allowed for local development — see
// httpsec.RequireSecureURL), since both carry credentials or a bearer
// token. That's the one point this diverges from
// integrations/csm-notification-service's own
// internal/notifications.EmailClient, whose constructor never fails.
type Client struct {
	http            *http.Client
	baseURL         string
	fromAddress     string
	debugRecipients []string
}

// NewClient constructs a Client that authenticates against the email
// notification service using the OAuth2 client credentials grant type.
func NewClient(cfg Config) (*Client, error) {
	// BaseURL is allowed to be empty — email is optional per deployment
	// (see this type's own doc comment); an empty BaseURL just means
	// SendEmail will fail the first time something actually tries to use
	// it, same as before this validation existed. TokenURL is never
	// optional (both clients share OAUTH2_TOKEN_URL, which cmd/server/main.go
	// requires via mustEnv regardless of whether email itself is
	// configured), so it's always checked.
	if err := httpsec.RequireSecureURL(cfg.TokenURL); err != nil {
		return nil, fmt.Errorf("notify: token URL: %w", err)
	}
	if cfg.BaseURL != "" {
		if err := httpsec.RequireSecureURL(cfg.BaseURL); err != nil {
			return nil, fmt.Errorf("notify: base URL: %w", err)
		}
	}

	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}

	tokenHTTPClient := &http.Client{Timeout: emailTokenFetchTimeout}
	httpsec.RejectInsecureRedirects(tokenHTTPClient)
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient, tokenHTTPClient)
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 25 * time.Second
	httpsec.RejectInsecureRedirects(httpClient)

	return &Client{
		http:            httpClient,
		baseURL:         strings.TrimRight(cfg.BaseURL, "/"),
		fromAddress:     cfg.FromAddress,
		debugRecipients: cleanAddresses(cfg.DebugRecipients),
	}, nil
}

// DebugMode reports whether debug mail mode is on (see
// Config.DebugRecipients).
func (c *Client) DebugMode() bool {
	return len(c.debugRecipients) > 0
}

// cleanAddresses trims the addresses and drops blanks, so a trailing
// comma in configuration never becomes an empty recipient.
func cleanAddresses(in []string) []string {
	var out []string
	for _, a := range in {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}

// testCopyNotice prepends the debug-mode notice to htmlBody: who the
// email would have gone to. It goes just inside <body> when the body has
// one, so a full HTML document keeps its head and styles, and at the very
// start otherwise.
func testCopyNotice(htmlBody string, to, cc []string) string {
	intended := "To: " + strings.Join(to, ", ")
	if len(cc) > 0 {
		intended += "; Cc: " + strings.Join(cc, ", ")
	}
	notice := `<p style="margin:0; padding:12px 16px; background-color:#fff4e5; color:#7a4100; font-family:Arial, Helvetica, sans-serif; font-size:13px;"><b>[Test copy]</b> Debug mail mode is on. This email would have been sent to ` +
		escapeHTML(intended) + `.</p>` + "\n"
	if i := strings.Index(strings.ToLower(htmlBody), "<body"); i >= 0 {
		if j := strings.Index(htmlBody[i:], ">"); j >= 0 {
			at := i + j + 1
			return htmlBody[:at] + "\n" + notice + htmlBody[at:]
		}
	}
	return notice + htmlBody
}

// sendEmailRequest is the wire shape expected by POST /send-email.
type sendEmailRequest struct {
	To          []string     `json:"to"`
	CC          []string     `json:"cc,omitempty"`
	From        string       `json:"from"`
	Subject     string       `json:"subject"`
	Template    []byte       `json:"template"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment is one file sent with an email: a calendar invitation, say.
type Attachment struct {
	// ContentName is the file name ("invite.ics").
	ContentName string
	// ContentType is the MIME type ("text/calendar; method=REQUEST; charset=UTF-8").
	ContentType string
	// Content is the raw file.
	Content []byte
}

// MarshalJSON writes the content as a JSON array of byte values. The email
// service binds it to a byte[] and refuses the base64 string encoding/json
// writes for a []byte ("should be of type 'byte[]'"); the template field is
// unaffected.
func (a Attachment) MarshalJSON() ([]byte, error) {
	values := make([]int, len(a.Content))
	for i, c := range a.Content {
		values[i] = int(c)
	}
	return json.Marshal(struct {
		ContentName string `json:"contentName"`
		ContentType string `json:"contentType"`
		Attachment  []int  `json:"attachment"`
	}{a.ContentName, a.ContentType, values})
}

// SendEmail sends a plain HTML email via the notification service. The
// sender address is always the client's configured FromAddress. A request
// with no recipients is a silent no-op — see registry.Task.To's own doc
// comment for why that's a deliberate, valid configuration rather than an
// error. cc may be nil; a nil/empty cc with a non-empty to still sends
// normally, just with no one cc'd.
func (c *Client) SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error {
	return c.SendEmailWithAttachments(ctx, to, cc, subject, htmlBody, nil)
}

// SendEmailWithAttachments is SendEmail with files attached.
func (c *Client) SendEmailWithAttachments(ctx context.Context, to, cc []string, subject, htmlBody string, attachments []Attachment) error {
	if len(to) == 0 {
		return nil
	}
	if subject == "" {
		return fmt.Errorf("notify: subject is required")
	}
	if c.DebugMode() {
		// Debug mail mode: the real audience is named in the body and
		// never addressed. The log line is the audit trail of what was
		// redirected; the addresses themselves stay out of the logs.
		slog.Info("debug mail mode: redirecting email to the debug recipients",
			"subject", subject, "intendedTo", len(to), "intendedCc", len(cc), "debugRecipients", len(c.debugRecipients))
		htmlBody = testCopyNotice(htmlBody, to, cc)
		subject = "[Test copy] " + subject
		to, cc = c.debugRecipients, nil
	}

	reqBody, err := json.Marshal(sendEmailRequest{
		To:          to,
		CC:          cc,
		From:        c.fromAddress,
		Subject:     subject,
		Template:    []byte(htmlBody),
		Attachments: attachments,
	})
	if err != nil {
		return fmt.Errorf("notify: encode send-email request: %w", err)
	}

	var reqReader io.Reader = bytes.NewReader(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/send-email", reqReader)
	if err != nil {
		return fmt.Errorf("notify: build send-email request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("notify: send-email: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("notify: read send-email response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := respBody
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}
	return nil
}
