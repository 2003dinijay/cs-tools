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

// Package twilio places voice calls through Twilio's REST API. It mirrors
// csm-notification-service's TwilioClient.MakeCall (separate Go modules, kept
// in sync by hand): HTTP Basic Auth with the account SID/auth token, and a
// <Say> TwiML document built locally.
package twilio

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIBaseURL = "https://api.twilio.com/2010-04-01"
	maxCallBody       = 4096
)

// Config holds the Twilio settings. AccountSID is an identifier, not a
// secret; AuthToken is the credential.
type Config struct {
	AccountSID string
	AuthToken  string
	// FromNumber is the Twilio-provisioned caller ID (E.164).
	FromNumber string
	// Voice and Language select <Say>'s text-to-speech; empty uses Twilio's
	// account defaults.
	Voice    string
	Language string
	// RingTimeoutSeconds is how long a call rings before Twilio gives up; zero
	// leaves Twilio's own default.
	RingTimeoutSeconds int
	// APIBaseURL overrides the REST base (regional edge, or a mock in tests).
	APIBaseURL string
}

// Client places calls.
type Client struct {
	http *http.Client
	cfg  Config
}

// Call is the call Twilio created: its sid and accepted status. The "to"
// number is deliberately not kept, so it never reaches a log line.
type Call struct {
	SID    string `json:"sid"`
	Status string `json:"status"`
}

// New constructs a Client, rejecting a configuration that cannot place a call.
func New(cfg Config) (*Client, error) {
	if cfg.AccountSID == "" || cfg.AuthToken == "" || cfg.FromNumber == "" {
		return nil, errors.New("twilio: TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN and TWILIO_FROM_NUMBER are required")
	}
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = defaultAPIBaseURL
	}
	return &Client{http: &http.Client{Timeout: 10 * time.Second}, cfg: cfg}, nil
}

type twimlResponse struct {
	XMLName xml.Name `xml:"Response"`
	Say     twimlSay `xml:"Say"`
}

type twimlSay struct {
	Voice    string `xml:"voice,attr,omitempty"`
	Language string `xml:"language,attr,omitempty"`
	Message  string `xml:",chardata"`
}

// sayTwiML builds the <Say> document through encoding/xml, so a message taken
// from an email subject is escaped and cannot inject other TwiML verbs.
func sayTwiML(message, voice, language string) (string, error) {
	doc, err := xml.Marshal(twimlResponse{Say: twimlSay{Voice: voice, Language: language, Message: message}})
	if err != nil {
		return "", fmt.Errorf("twilio: encode message as TwiML: %w", err)
	}
	return xml.Header + string(doc), nil
}

// MakeCall places a voice call to `to` (E.164) that reads message aloud.
func (c *Client) MakeCall(ctx context.Context, to, message string) (Call, error) {
	if strings.TrimSpace(to) == "" || strings.TrimSpace(message) == "" {
		return Call{}, errors.New("twilio: to and message are required")
	}
	twiml, err := sayTwiML(message, c.cfg.Voice, c.cfg.Language)
	if err != nil {
		return Call{}, err
	}
	form := url.Values{"To": {to}, "From": {c.cfg.FromNumber}, "Twiml": {twiml}}
	if c.cfg.RingTimeoutSeconds > 0 {
		form.Set("Timeout", strconv.Itoa(c.cfg.RingTimeoutSeconds))
	}

	endpoint := c.cfg.APIBaseURL + "/Accounts/" + url.PathEscape(c.cfg.AccountSID) + "/Calls.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Call{}, fmt.Errorf("twilio: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.cfg.AccountSID, c.cfg.AuthToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return Call{}, fmt.Errorf("twilio: call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		// Status only: the body can echo the destination number.
		return Call{}, fmt.Errorf("twilio: call rejected with status %d", resp.StatusCode)
	}
	// The call is already accepted; an unreadable body degrades to an empty
	// Call, never to a failure.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCallBody))
	if err != nil {
		return Call{}, nil
	}
	var created Call
	if json.Unmarshal(body, &created) != nil {
		return Call{}, nil
	}
	return created, nil
}
