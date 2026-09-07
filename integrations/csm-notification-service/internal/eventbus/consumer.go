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

package eventbus

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

// handleAttempts is how many times a single record's Handle func is called
// in total before giving up on it — not additional retries on top of a first
// call, so handleAttempts=3 means 3 calls, 2 of them retries. A record that
// still fails after this many attempts is handed to OnExhausted (see Run) —
// typically published to a dead-letter topic rather than dropped — and its
// offset is committed anyway either way, so one permanently-failing record
// (e.g. a downstream outage) cannot block every later record on its
// partition forever. The DLQ topic's own Consumer uses the same
// handleAttempts/handleRetryDelay for its own retry pass over a
// dead-lettered record (see cmd/server/main.go) — there is deliberately no
// third tier past that: an OnExhausted of nil there just logs and drops.
const handleAttempts = 3

// handleRetryDelay is the fixed pause between attempts. This is deliberately
// simple (no exponential backoff) since handleAttempts is small and this is
// covering transient blips (a downstream timeout), not sustained outages.
const handleRetryDelay = 2 * time.Second

// Record is the eventbus-agnostic view of a consumed message that Handle
// receives — deliberately not the underlying Kafka client's own message
// type, so dispatch (and any future caller) never needs to import
// github.com/segmentio/kafka-go directly.
type Record struct {
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
	// IsFinalAttempt is true on the last of handleAttempts calls to Handle
	// for this record on THIS topic — set by processRecord, which is about
	// to call onExhausted (or log-and-drop) and move on regardless of this
	// call's outcome. This does not by itself mean no further attempt will
	// ever come for this event's content: the main consumer's onExhausted
	// republishes the exact same Value to the dead-letter topic, which gets
	// its own fresh IsFinalAttempt cycle. See NoMoreRetries for the
	// property a Handle implementation actually wants when deciding whether
	// it's safe to release content-keyed idempotency state.
	IsFinalAttempt bool
	// NoMoreRetries is true only when there is no further tier of retries
	// coming for this event's content, on any topic. It's set two ways:
	// on IsFinalAttempt when onExhausted is nil (the DLQ topic's own
	// Consumer always has onExhausted=nil — see cmd/server/main.go — so
	// this is true on its final attempt; the main topic's Consumer has
	// onExhausted set, so its own final attempt leaves this false, since
	// one more tier of attempts is coming on the DLQ topic); or on the
	// extra cleanup call processRecord makes when onExhausted itself
	// fails (the dead-letter publish didn't go through, so the DLQ tier
	// that would otherwise be coming never will — this really is the end
	// of the line for this content, not just this topic's own attempts).
	//
	// A Handle that keys its own idempotency tracking off event content
	// rather than Kafka coordinates (see dispatch.recordBaseKey) must gate
	// releasing that tracking on this, not IsFinalAttempt: releasing on the
	// main topic's final attempt would let an already-succeeded channel be
	// reclaimed and resent once the dead-lettered record's own first
	// attempt arrives, since it would compute the exact same content key.
	NoMoreRetries bool
}

// Consumer reads records from a topic as a member of a named consumer group,
// so multiple running instances of this service split the topic's partitions
// between them instead of each seeing every record.
type Consumer struct {
	reader *kafka.Reader
}

// NewConsumer constructs a Consumer that joins groupID and consumes
// cfg.Topic. Auto-commit is not used: offsets are committed explicitly by
// Run, only after a record has been handled (or exhausted its retries) —
// never before, so a crash mid-processing redelivers the record on restart
// instead of silently skipping it.
func NewConsumer(cfg Config, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{cfg.Broker},
			GroupID: groupID,
			Topic:   cfg.Topic,
			Dialer: &kafka.Dialer{
				TLS:           &tls.Config{MinVersion: tls.VersionTLS12},
				SASLMechanism: cfg.saslMechanism(),
			},
			// Only applies to a partition with no committed offset yet (e.g.
			// the very first time this consumer group ever runs) — this is
			// kafka-go's own default, set explicitly here for clarity and to
			// document the reason: a notification service should process
			// backlog, not silently start from the tail. The Kafka client
			// used before this one defaulted the other way and needed this
			// set explicitly to avoid dropping events published just before
			// its first join — confirmed against the real namespace.
			StartOffset: kafka.FirstOffset,
			Logger:      kafka.LoggerFunc(logDebug),
			ErrorLogger: kafka.LoggerFunc(logError),
			// kafka-go's consumer-group rebalancing only offers Range and
			// RoundRobin balancers (its default, left unset here) — there is
			// no cooperative/incremental strategy like the Kafka client used
			// before this one had. In practice this only matters once this
			// service scales beyond one instance: a rebalance briefly pauses
			// every partition in the group instead of only the ones actually
			// moving. Not a concern for a single running instance.
		}),
	}
}

// Handle processes a single record. A non-nil error causes Run to retry (see
// handleAttempts).
type Handle func(context.Context, Record) error

// OnExhausted is called once a record's Handle call has failed on every one
// of handleAttempts attempts. Run commits the record's offset right after
// this call returns regardless of its outcome — either way nothing will
// attempt this record again on this topic, so there's nothing left to gate
// the commit on.
//
// Passing nil (as the DLQ consumer's own Run call does — see cmd/server/
// main.go) falls back to logging the failure at ERROR and dropping the
// record — the right default for a queue with nowhere further to escalate
// to. The main consumer instead passes a func that publishes the record to
// the dead-letter topic, so a persistently-failing record (e.g. a downstream
// outage) doesn't just vanish once its content ages out of Event Hub's own
// retention window.
//
// A non-nil return only affects logging (Run logs it at ERROR alongside the
// original handleErr) — it does not change whether Run commits, since a
// failure here (e.g. the dead-letter topic itself is unreachable) has no
// lower tier to fall back to either.
type OnExhausted func(ctx context.Context, record Record, handleErr error) error

// Run polls for records and calls handle for each one, committing its offset
// once handle succeeds or its retries are exhausted. Run blocks until ctx is
// canceled or the Consumer is closed; call it from its own goroutine.
func (c *Consumer) Run(ctx context.Context, handle Handle, onExhausted OnExhausted) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) {
				return
			}
			slog.ErrorContext(ctx, "eventbus: fetch error", "err", err)
			continue
		}
		record := Record{
			Topic:     msg.Topic,
			Partition: msg.Partition,
			Offset:    msg.Offset,
			Key:       msg.Key,
			Value:     msg.Value,
		}
		if !processRecord(ctx, record, handle, onExhausted, handleAttempts, handleRetryDelay) {
			// ctx was canceled mid-retry-wait (shutdown) — skip the commit,
			// same as the fetch loop above; the next FetchMessage call will
			// see ctx.Err() != nil and return.
			continue
		}
		if cerr := c.reader.CommitMessages(ctx, msg); cerr != nil {
			slog.ErrorContext(ctx, "eventbus: commit failed", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", cerr)
		}
	}
}

// processRecord calls handle up to attempts times (pausing retryDelay
// between attempts), then, if every attempt failed, calls onExhausted (or
// logs and drops if onExhausted is nil). Factored out of Run so the
// retry/escalation logic is testable without a real Kafka broker — it never
// touches c.reader itself.
//
// Returns whether Run should commit the record's offset afterward — false
// only when ctx was canceled mid-retry-wait (a shutdown in progress), so a
// record that was never actually finished being handled isn't marked done.
func processRecord(ctx context.Context, record Record, handle Handle, onExhausted OnExhausted, attempts int, retryDelay time.Duration) bool {
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		record.IsFinalAttempt = attempt == attempts
		record.NoMoreRetries = record.IsFinalAttempt && onExhausted == nil
		if err = handle(ctx, record); err == nil {
			return true
		}
		slog.ErrorContext(ctx, "eventbus: handler failed",
			"topic", record.Topic, "partition", record.Partition, "offset", record.Offset,
			"attempt", attempt, "maxAttempts", attempts, "err", err)
		if attempt < attempts {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(retryDelay):
			}
		}
	}
	if onExhausted != nil {
		if dlqErr := onExhausted(ctx, record, err); dlqErr != nil {
			slog.ErrorContext(ctx, "eventbus: dead-letter publish also failed; record is now unrecoverable outside Event Hub's retention window",
				"topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "handleErr", err, "onExhaustedErr", dlqErr)
			// The DLQ publish itself failed, so — unlike the ordinary
			// exhaustion case below NoMoreRetries deliberately stays false
			// for — there is truly no future delivery of this content
			// coming on any topic; this record is being dropped for good.
			// Call handle once more with NoMoreRetries now true, purely so
			// a Handle that tracks content-keyed idempotency state (see
			// dispatch.Dispatcher) gets a chance to release it — without
			// this, it would never learn this record is done and would
			// leak that state forever. Any channel already claimed is a
			// no-op here (claim() rejects a key already held); this call's
			// own error changes nothing about the commit decision below
			// (the record is being dropped either way) but is still logged.
			record.NoMoreRetries = true
			if cleanupErr := handle(ctx, record); cleanupErr != nil {
				slog.ErrorContext(ctx, "eventbus: final cleanup handle call after dead-letter failure returned an error (ignored — record is being dropped)",
					"topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", cleanupErr)
			}
		}
		return true
	}
	slog.ErrorContext(ctx, "eventbus: handler exhausted retries, dropping record",
		"topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", err)
	return true
}

// Close leaves the consumer group and closes the underlying connection.
func (c *Consumer) Close() {
	_ = c.reader.Close()
}
