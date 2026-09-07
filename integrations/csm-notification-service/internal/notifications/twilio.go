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

package notifications

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
)

// defaultTwilioAPIBaseURL is Twilio's REST API base, used whenever
// TwilioConfig.APIBaseURL isn't set.
const defaultTwilioAPIBaseURL = "https://api.twilio.com/2010-04-01"

// TwilioConfig holds the configuration for the SMS notification channel.
// AccountSID is Twilio's own identifier for the account, not a secret (it
// appears in the request URL and in Twilio's own webhooks) — only AuthToken
// needs the same care as any other credential.
type TwilioConfig struct {
	AccountSID string
	AuthToken  string
	// APIBaseURL overrides Twilio's REST API base
	// (defaultTwilioAPIBaseURL). Empty uses that default — only set this for
	// a regional Twilio edge/API endpoint, or to point at a mock server in
	// tests.
	APIBaseURL string
	// FromNumber is a fixed Twilio-provisioned sending number (E.164, e.g.
	// "+14155552671"), used two ways: as SendSMS's fallback sender when
	// MessagingServiceSid isn't set, and as MakeCall's caller ID — Twilio
	// Voice has no MessagingServiceSid equivalent, so FromNumber is required
	// for MakeCall regardless of whether MessagingServiceSid is configured.
	FromNumber string
	// MessagingServiceSid selects a Twilio Messaging Service (a sender pool
	// that Twilio itself picks a number from, plus opt-out/compliance
	// handling) instead of one fixed number, for SendSMS only. This is how
	// our Twilio account actually sends SMS today — prefer it over
	// FromNumber for any real deployment.
	MessagingServiceSid string
	// Voice selects the text-to-speech voice MakeCall's <Say> uses (e.g.
	// "Polly.Raveena", "Polly.Aditi", "man", "woman") — Twilio's full list:
	// https://www.twilio.com/docs/voice/twiml/say/text-speech#available-voices-and-languages.
	// Empty uses Twilio's own account default voice.
	Voice string
	// Language sets <Say>'s language/locale (e.g. "en-IN", "en-GB"), which
	// affects pronunciation — worth setting explicitly for names/terms the
	// default voice's default locale might mispronounce. Empty uses
	// Twilio's own default for the selected Voice.
	Language string
}

// TwilioClient sends SMS messages via Twilio's REST API, authenticated with
// HTTP Basic Auth (AccountSID/AuthToken) — Twilio has no OAuth2 flow.
//
// NewTwilioClient never fails, so it is safe to construct with a zero-value
// TwilioConfig (e.g. when this channel is not yet configured for a given
// deployment) — a missing configuration only surfaces as an error the first
// time SendSMS is called.
type TwilioClient struct {
	http *http.Client
	cfg  TwilioConfig
}

// NewTwilioClient constructs a TwilioClient from cfg.
func NewTwilioClient(cfg TwilioConfig) *TwilioClient {
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = defaultTwilioAPIBaseURL
	}
	return &TwilioClient{
		http: &http.Client{Timeout: 10 * time.Second},
		cfg:  cfg,
	}
}

// SendSMS sends a single SMS to `to` (E.164, e.g. "+14155552671") with the
// given body text, from the account's configured MessagingServiceSid (if
// set) or else its fixed FromNumber.
func (c *TwilioClient) SendSMS(ctx context.Context, to, body string) error {
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("notifications: to is required")
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("notifications: body is required")
	}
	if c.cfg.AccountSID == "" || c.cfg.AuthToken == "" ||
		(c.cfg.MessagingServiceSid == "" && c.cfg.FromNumber == "") {
		return fmt.Errorf("notifications: twilio is not configured")
	}

	form := url.Values{
		"To":   {to},
		"Body": {body},
	}
	if c.cfg.MessagingServiceSid != "" {
		form.Set("MessagingServiceSid", c.cfg.MessagingServiceSid)
	} else {
		form.Set("From", c.cfg.FromNumber)
	}

	return c.do(ctx, "Messages.json", form)
}

// MakeCall places a single voice call to `to` (E.164, e.g. "+14155552671")
// that reads `message` aloud via Twilio's text-to-speech (a TwiML <Say>
// document built locally — no external TwiML hosting needed), using the
// client's configured Voice/Language, from the account's configured
// FromNumber. Unlike SendSMS, this always requires FromNumber: Twilio Voice
// has no MessagingServiceSid equivalent.
func (c *TwilioClient) MakeCall(ctx context.Context, to, message string) error {
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("notifications: to is required")
	}
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("notifications: message is required")
	}
	if c.cfg.AccountSID == "" || c.cfg.AuthToken == "" || c.cfg.FromNumber == "" {
		return fmt.Errorf("notifications: twilio is not configured")
	}

	twiml, err := sayTwiML(message, c.cfg.Voice, c.cfg.Language)
	if err != nil {
		return err
	}
	form := url.Values{
		"To":    {to},
		"From":  {c.cfg.FromNumber},
		"Twiml": {twiml},
	}

	return c.do(ctx, "Calls.json", form)
}

// twimlResponse is the <Response><Say voice="..." language="...">message</Say></Response>
// document MakeCall sends as Twilio's Twiml param. Built via encoding/xml's
// Marshal (struct fields, not string concatenation) so both the message
// content and the Voice/Language attributes get correct XML escaping for
// free — message in particular is caller-supplied, so treating it as
// literal XML would let it inject arbitrary TwiML (e.g. a <Dial> or
// <Redirect> verb) instead of just being spoken.
type twimlResponse struct {
	XMLName xml.Name `xml:"Response"`
	Say     twimlSay `xml:"Say"`
}

type twimlSay struct {
	Voice    string `xml:"voice,attr,omitempty"`
	Language string `xml:"language,attr,omitempty"`
	Message  string `xml:",chardata"`
}

// sayTwiML builds a TwiML document that reads message aloud via Twilio's
// <Say> verb, in the given voice/language (either may be empty to use
// Twilio's own account defaults — see TwilioConfig.Voice/Language for
// examples).
func sayTwiML(message, voice, language string) (string, error) {
	doc, err := xml.Marshal(twimlResponse{
		Say: twimlSay{Voice: voice, Language: language, Message: message},
	})
	if err != nil {
		return "", fmt.Errorf("notifications: encode call message as TwiML: %w", err)
	}
	return xml.Header + string(doc), nil
}

// do POSTs a form-encoded request to the given Twilio resource path (e.g.
// "Messages.json", "Calls.json") under this account, authenticated with
// Basic Auth, and maps a non-201 response to *apierror.Error.
func (c *TwilioClient) do(ctx context.Context, resourcePath string, form url.Values) error {
	endpoint := c.cfg.APIBaseURL + "/Accounts/" + url.PathEscape(c.cfg.AccountSID) + "/" + resourcePath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("notifications: build twilio request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.cfg.AccountSID, c.cfg.AuthToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("notifications: call twilio: %w", err)
	}
	defer resp.Body.Close()

	// Twilio returns 201 Created for both a successfully queued message and
	// a successfully initiated call — the body is only needed on failure, so
	// skip reading it here rather than reading a response we're about to
	// discard.
	if resp.StatusCode == http.StatusCreated {
		return nil
	}

	// Bounded even on read failure: io.LimitReader caps how much of a
	// misbehaving (or unexpectedly huge) error response this ever buffers in
	// memory, rather than reading the full body before truncating it.
	const maxErrBody = 256
	excerpt, err := io.ReadAll(io.LimitReader(resp.Body, maxErrBody))
	if err != nil {
		return fmt.Errorf("notifications: read twilio response: %w", err)
	}
	return &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
}
