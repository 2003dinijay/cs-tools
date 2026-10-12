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

// Package novera tells Novera, the Google Chat assistant, about a new entry.
// Novera sends it to every employee who has a chat open with it.
package novera

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/wso2-open-operations/cs-tools/apps/til-backend/backend/internal/entry"
)

// Config is how to reach Novera's notification endpoint. The API gateway in
// front of it takes a client-credentials token from an application
// subscribed to Novera's API.
type Config struct {
	NotifyURL    string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Client sends new-entry notifications.
type Client struct {
	url  string
	http *http.Client
}

// New returns a Client for cfg.
func New(cfg Config) *Client {
	cc := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       cfg.Scopes,
	}
	tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Timeout: 10 * time.Second})
	httpClient := cc.Client(tokenCtx)
	httpClient.Timeout = 10 * time.Second
	return &Client{url: cfg.NotifyURL, http: httpClient}
}

type payload struct {
	Title       string  `json:"title"`
	Who         string  `json:"who"`
	Where       string  `json:"where"`
	WhereDetail *string `json:"whereDetail"`
	What        string  `json:"what"`
	EntryURL    *string `json:"entryUrl"`
}

// Notify announces e. entryURL, when not empty, is the link the card opens.
// Best effort: the entry is already saved, so a failure here is logged and
// never reaches the person who shared it.
func (c *Client) Notify(ctx context.Context, e entry.Entry, entryURL string) {
	p := payload{
		// The card puts these straight into its text, so they are escaped
		// here rather than relying on Novera to do it.
		Title:       html.EscapeString(e.Title),
		Who:         html.EscapeString(e.Who),
		Where:       e.Where,
		WhereDetail: e.WhereDetail,
		What:        entry.ForChat(e.What),
	}
	if entryURL != "" {
		p.EntryURL = &entryURL
	}
	body, err := json.Marshal(p)
	if err != nil {
		slog.ErrorContext(ctx, "novera: encode notification", "err", err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		slog.ErrorContext(ctx, "novera: build notification", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		slog.ErrorContext(ctx, "novera: send notification", "entryID", e.ID, "err", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		slog.ErrorContext(ctx, "novera: notification rejected", "entryID", e.ID, "status", resp.StatusCode)
	}
}
