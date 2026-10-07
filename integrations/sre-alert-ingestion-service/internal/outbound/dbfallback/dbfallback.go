// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package dbfallback posts alerts that could not be written to Postgres straight to one Google Chat space, so a database outage still reaches someone.
package dbfallback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"

	"sre-alert-ingestion-service/internal/model"
)

const (
	// maxPending caps alerts waiting for the next message; more are only counted.
	maxPending = 500
	// maxListed caps alerts rendered in one card; the rest are summarised as a count.
	maxListed = 20
	// maxDescription truncates each alert's description, in runes.
	maxDescription = 300
	// sendGap spaces messages so an outage coalesces into one card per gap instead of tripping Chat's per-space rate limit.
	sendGap = 10 * time.Second
	// sendAttempts and retryBaseDelay retry 429 and 5xx answers; other 4xx answers are not retried.
	sendAttempts   = 3
	retryBaseDelay = 500 * time.Millisecond
)

type entry struct {
	source    string
	requestID string
	alert     model.Alert
}

// Client sends at most one message at a time, merging alerts reported while one is in flight or during sendGap into the next.
type Client struct {
	logger  *slog.Logger
	url     string
	spaceID string
	http    *http.Client
	gap     time.Duration

	mu      sync.Mutex
	pending []entry
	dropped int
	running bool
	idle    *sync.Cond

	stop     chan struct{}
	stopOnce sync.Once
}

// New returns a Client for webhookURL, which must be https since it carries the space's key and token.
func New(logger *slog.Logger, webhookURL string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(webhookURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		// The URL is a credential, so it is never echoed back.
		return nil, errors.New("DB_FALLBACK_CHAT_WEBHOOK_URL must be an https Google Chat webhook URL")
	}
	// Redirects are never followed, so the key and token only go to the configured host.
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	c := &Client{
		logger:  logger,
		url:     webhookURL,
		spaceID: spaceID(webhookURL),
		http:    client,
		gap:     sendGap,
		stop:    make(chan struct{}),
	}
	c.idle = sync.NewCond(&c.mu)
	return c, nil
}

// Notify queues alerts for the Chat space without blocking.
func (c *Client) Notify(source, requestID string, alerts []model.Alert) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, a := range alerts {
		if len(c.pending) >= maxPending {
			c.dropped++
			continue
		}
		c.pending = append(c.pending, entry{source: source, requestID: requestID, alert: a})
	}
	if !c.running {
		c.running = true
		go c.loop()
	}
}

func (c *Client) loop() {
	for {
		c.mu.Lock()
		batch, dropped := c.pending, c.dropped
		c.pending, c.dropped = nil, 0
		if len(batch) == 0 {
			c.running = false
			c.idle.Broadcast()
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		c.send(batch, dropped)

		t := time.NewTimer(c.gap)
		select {
		case <-t.C:
		case <-c.stop:
		}
		t.Stop()
	}
}

// Close skips the remaining gap so queued alerts go out now, then waits for them or ctx; used on shutdown.
func (c *Client) Close(ctx context.Context) {
	c.stopOnce.Do(func() { close(c.stop) })
	done := make(chan struct{})
	go func() {
		c.mu.Lock()
		for c.running {
			c.idle.Wait()
		}
		c.mu.Unlock()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		c.logger.Warn("db fallback chat did not finish before shutdown", "chat_space_id", c.spaceID)
	}
}

func (c *Client) send(batch []entry, dropped int) {
	body, err := json.Marshal(card(batch, dropped))
	if err != nil {
		c.logger.Error("db fallback chat card could not be built", "alerts", len(batch)+dropped, "error", err)
		return
	}
	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = retryBaseDelay
	_, err = backoff.Retry(context.Background(), func() (struct{}, error) {
		status, err := c.post(body)
		if err != nil && status >= 400 && status < 500 && status != http.StatusTooManyRequests {
			return struct{}{}, backoff.Permanent(err)
		}
		return struct{}{}, err
	}, backoff.WithBackOff(eb), backoff.WithMaxTries(sendAttempts))
	if err != nil {
		c.logger.Error("db fallback chat failed; these alerts were neither stored nor posted", "chat_space_id", c.spaceID, "alerts", len(batch)+dropped, "error", err)
		return
	}
	c.logger.Info("db fallback chat sent", "chat_space_id", c.spaceID, "alerts", len(batch)+dropped)
}

// post returns status 0 when the request never got a response.
func (c *Client) post(body []byte) (int, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return 0, errors.New("build request")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) {
			// Strip url.Error's embedded URL so the webhook's key and token don't reach logs.
			return 0, fmt.Errorf("%s request failed: %w", uerr.Op, uerr.Err)
		}
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return resp.StatusCode, fmt.Errorf("status %d: %s", resp.StatusCode, bytes.TrimSpace(snippet))
	}
	return resp.StatusCode, nil
}

// spaceID names the space in logs without the URL's embedded credential.
func spaceID(webhookURL string) string {
	_, rest, ok := strings.Cut(webhookURL, "/spaces/")
	if !ok {
		return "unknown"
	}
	if id, _, ok := strings.Cut(rest, "/"); ok {
		return id
	}
	return "unknown"
}

// card renders up to maxListed alerts; every alert field is escaped since it comes from the webhook sender.
func card(batch []entry, dropped int) map[string]any {
	total := len(batch) + dropped
	widgets := []map[string]any{paragraph(fmt.Sprintf(
		"%d alert(s) could not be written to the database, so no incidents will be created for them. Senders were answered 503 and may resend.", total))}
	for i, e := range batch {
		if i == maxListed {
			break
		}
		widgets = append(widgets, paragraph(alertText(e)))
	}
	if more := total - min(len(batch), maxListed); more > 0 {
		widgets = append(widgets, paragraph(fmt.Sprintf("<i>%d more not shown.</i>", more)))
	}
	return map[string]any{
		"cardsV2": []map[string]any{{
			"cardId": "db-fallback",
			"card": map[string]any{
				"header": map[string]any{
					"title":    "<font color='#f70707'><b>DB FALLBACK | Alerts not stored</b></font>",
					"subtitle": fmt.Sprintf("%d alert(s) received by alert ingestion", total),
				},
				"sections": []map[string]any{{"widgets": widgets}},
			},
		}},
	}
}

func paragraph(text string) map[string]any {
	return map[string]any{"textParagraph": map[string]any{"text": text}}
}

func alertText(e entry) string {
	a := e.alert
	var b strings.Builder
	b.WriteString("<b>" + html.EscapeString(orDash(a.Severity)) + " | " + html.EscapeString(orDash(a.Service)) + "</b>")
	for _, f := range []struct{ name, value string }{
		{"Metric", a.MetricName},
		{"Environment", a.Environment},
		{"Category", a.Category},
		{"Source", e.source},
		{"Unique ID", a.UniqueIdentifier},
		{"Description", truncate(a.Description, maxDescription)},
		{"Request ID", e.requestID},
	} {
		if f.value != "" {
			b.WriteString("<br><b>" + f.name + ":</b> " + html.EscapeString(f.value))
		}
	}
	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
