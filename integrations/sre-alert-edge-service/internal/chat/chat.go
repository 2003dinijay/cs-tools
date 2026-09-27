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

// Package chat posts Google Chat cards for the two things only a person can act on here: a
// webhook we rejected (so the sender can be fixed), and an alert we accepted but couldn't
// store (so no incident was created). Card layout follows sre-alert-core-service's cards.
//
// Posting is asynchronous and rate limited, so it never slows a request or a writer. When
// Chat itself fails, the details are logged at ERROR instead.
package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"sync"
	"time"
	"unicode/utf8"

	"sre-alert-edge-service/internal/allocator"
	"sre-alert-edge-service/internal/server"
)

// Settings tunes the cards; see config.toml.example.
type Settings struct {
	// RejectWindow: the first rejection per vendor+error in each window posts a card; later
	// ones in the window are counted and reported on the next card.
	RejectWindow time.Duration
	// BodyPreviewChars bounds how much of a rejected body the card shows.
	BodyPreviewChars int
	// CardsPerMinute caps DB-failure cards; beyond it, one summary is posted per interval.
	CardsPerMinute int
	// SummaryInterval is the DB-failure rate-limit window (one minute in production).
	SummaryInterval time.Duration
	HTTPTimeout     time.Duration
}

// Notifier implements server.RejectNotifier and allocator.FailureNotifier.
type Notifier struct {
	logger   *slog.Logger
	urls     []string
	replica  string
	settings Settings
	http     *http.Client
	now      func() time.Time

	mu      sync.Mutex
	rejects map[string]*rejectState
	db      dbState

	sends sync.WaitGroup
	stop  chan struct{}
	done  chan struct{}
}

type rejectState struct {
	lastCard   time.Time
	suppressed int
}

type dbState struct {
	sent            int // cards posted in the current interval
	suppressed      int
	suppressedSince time.Time
}

// New starts the DB-failure summary loop. Empty urls logs a warning and posts nothing (local
// dev); rejections and failures are still logged by their callers.
func New(logger *slog.Logger, urls []string, replica string, s Settings) *Notifier {
	if len(urls) == 0 {
		logger.Warn("FALLBACK_CHAT_WEBHOOK_URLS not set; rejected-webhook and DB-failure cards are disabled")
	}
	n := &Notifier{
		logger:   logger,
		urls:     urls,
		replica:  replica,
		settings: s,
		http:     &http.Client{Timeout: s.HTTPTimeout},
		now:      time.Now,
		rejects:  map[string]*rejectState{},
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go n.summaryLoop()
	return n
}

// Rejected posts the rejected-webhook card, at most once per vendor+error per RejectWindow.
func (n *Notifier) Rejected(r server.Rejection) {
	now := n.now().UTC()
	key := r.Vendor + "|" + r.Error

	n.mu.Lock()
	st := n.rejects[key]
	if st != nil && now.Sub(st.lastCard) < n.settings.RejectWindow {
		st.suppressed++
		n.mu.Unlock()
		return
	}
	var suppressed int
	var since time.Time
	if st != nil {
		suppressed, since = st.suppressed, st.lastCard
	}
	n.rejects[key] = &rejectState{lastCard: now}
	n.mu.Unlock()

	n.post(rejectedCard(r, now, n.replica, n.settings.BodyPreviewChars, suppressed, since),
		"rejected-webhook card not delivered", "vendor", r.Vendor, "request_id", r.RequestID, "error", r.Error)
}

// StoreFailed posts the DB-failure card, up to CardsPerMinute per interval; the rest are
// rolled into one summary card at the end of the interval.
func (n *Notifier) StoreFailed(f allocator.StoreFailure) {
	n.mu.Lock()
	if n.db.sent >= n.settings.CardsPerMinute {
		if n.db.suppressed == 0 {
			n.db.suppressedSince = n.now().UTC()
		}
		n.db.suppressed++
		n.mu.Unlock()
		return
	}
	n.db.sent++
	n.mu.Unlock()

	// The full alert goes in the fallback log line: if Chat fails too, the log is all that's left.
	n.post(dbFailureCard(f, n.now().UTC(), n.replica), "DB-failure card not delivered; alert NOT stored",
		"vendor", f.Vendor, "alt_id", f.AltID, "request_id", f.RequestID, "alert", f.Alert, "store_error", f.Err)
}

func (n *Notifier) summaryLoop() {
	defer close(n.done)
	ticker := time.NewTicker(n.settings.SummaryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			n.flushSummary()
		case <-n.stop:
			n.flushSummary()
			return
		}
	}
}

func (n *Notifier) flushSummary() {
	n.mu.Lock()
	count, since := n.db.suppressed, n.db.suppressedSince
	n.db = dbState{}
	n.mu.Unlock()
	if count > 0 {
		n.post(dbSummaryCard(count, since, n.replica), "DB-failure summary card not delivered",
			"suppressed_failures", count, "since", since)
	}
}

// Close posts any pending DB-failure summary and waits for in-flight posts or ctx.
func (n *Notifier) Close(ctx context.Context) {
	close(n.stop)
	select {
	case <-n.done:
	case <-ctx.Done():
		return
	}
	sent := make(chan struct{})
	go func() {
		n.sends.Wait()
		close(sent)
	}()
	select {
	case <-sent:
	case <-ctx.Done():
	}
}

// post sends card to every webhook in the background, logging failLog + attrs at ERROR if a
// webhook doesn't accept it.
func (n *Notifier) post(card map[string]any, failLog string, attrs ...any) {
	if len(n.urls) == 0 {
		return
	}
	body, err := json.Marshal(card)
	if err != nil {
		n.logger.Error(failLog, append(attrs, "chat_error", err)...)
		return
	}
	for _, url := range n.urls {
		n.sends.Add(1)
		go func() {
			defer n.sends.Done()
			if err := n.send(url, body); err != nil {
				n.logger.Error(failLog, append(attrs, "chat_error", err)...)
			}
		}()
	}
}

func (n *Notifier) send(url string, body []byte) error {
	resp, err := n.http.Post(url, "application/json; charset=UTF-8", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("chat returned %d", resp.StatusCode)
	}
	return nil
}

const timeLayout = "2006-01-02 15:04:05 UTC"

func rejectedCard(r server.Rejection, now time.Time, replica string, previewChars, suppressed int, since time.Time) map[string]any {
	details := line("HTTP status", fmt.Sprint(r.Status)) +
		line("Error", r.Error) +
		line("Route", r.Route) +
		line("Request id", r.RequestID) +
		line("Time", now.Format(timeLayout)) +
		line("Replica", replica) +
		line("Remote address", r.RemoteAddr) +
		line("Content type", r.ContentType) +
		line("Body size", fmt.Sprintf("%d bytes", r.BodySize))
	if suppressed > 0 {
		details += line("Also rejected", fmt.Sprintf("+%d more since %s", suppressed, since.Format(timeLayout)))
	}
	preview, truncated := truncate(string(r.Body), previewChars)
	if truncated {
		preview += " …"
	}
	if preview == "" {
		preview = "(empty body)"
	}
	return cardsV2("rejected-"+r.RequestID,
		"<font color='#f70707'><b>Rejected webhook: "+html.EscapeString(r.Vendor)+"</b></font>",
		fmt.Sprintf("HTTP %d | no alert id claimed, nothing stored", r.Status),
		section("", details),
		section(fmt.Sprintf("Body preview (first %d chars)", previewChars), html.EscapeString(preview)),
	)
}

func dbFailureCard(f allocator.StoreFailure, now time.Time, replica string) map[string]any {
	filler := "written; alerts-core skips this id"
	if !f.FillerWritten {
		filler = "NOT written; alerts-core will wait gap_timeout on this id"
	}
	a := f.Alert
	description, _ := truncate(a.Description, 500)
	return cardsV2("db-failure-"+f.AltID,
		"<font color='#f70707'><b>DB failure: alert NOT stored</b></font>",
		html.EscapeString(f.Vendor)+" | "+f.AltID,
		section("", "<b>No incident was created and CSM was not notified.</b><br>"+
			line("Vendor", f.Vendor)+
			line("Alert id", f.AltID)+
			line("Error", errString(f.Err))+
			line("Filler row", filler)+
			line("Request id", f.RequestID)+
			line("Replica", replica)+
			line("Time", now.Format(timeLayout))),
		section("Alert",
			line("Service", a.Service)+
				line("Metric name", a.MetricName)+
				line("Severity", a.Severity)+
				line("Category", a.Category)+
				line("Environment", a.Environment)+
				line("Source", a.Source)+
				line("Unique identifier", a.UniqueIdentifier)+
				line("Description", description)),
	)
}

func dbSummaryCard(count int, since time.Time, replica string) map[string]any {
	return cardsV2(fmt.Sprintf("db-failure-summary-%d", since.Unix()),
		"<font color='#f70707'><b>DB failures: "+fmt.Sprint(count)+" more alerts NOT stored</b></font>",
		"since "+since.Format(timeLayout),
		section("", fmt.Sprintf("<b>%d more alerts</b> could not be stored since %s, beyond the per-minute card limit. "+
			"No incidents were created for them. Full details are in the logs of replica %s.",
			count, since.Format(timeLayout), html.EscapeString(replica))),
	)
}

// line renders one "<b>Label:</b> value" row, escaping the value (it may come from a vendor).
func line(label, value string) string {
	if value == "" {
		value = "-"
	}
	return "<b>" + label + ":</b> " + html.EscapeString(value) + "<br>"
}

func section(header, text string) map[string]any {
	s := map[string]any{"widgets": []map[string]any{{"textParagraph": map[string]any{"text": text}}}}
	if header != "" {
		s["header"] = header
	}
	return s
}

func cardsV2(id, title, subtitle string, sections ...map[string]any) map[string]any {
	return map[string]any{
		"cardsV2": []map[string]any{{
			"cardId": id,
			"card": map[string]any{
				"header":   map[string]any{"title": title, "subtitle": subtitle},
				"sections": sections,
			},
		}},
	}
}

// truncate cuts s to n characters without splitting a multi-byte one.
func truncate(s string, n int) (string, bool) {
	if utf8.RuneCountInString(s) <= n {
		return s, false
	}
	r := []rune(s)
	return string(r[:n]), true
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
