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
	"errors"
	"testing"
	"time"
)

// testRetryDelay is short so exhaustion tests don't actually wait
// handleRetryDelay (2s) between attempts.
const testRetryDelay = time.Millisecond

func TestProcessRecord_SucceedsFirstAttempt(t *testing.T) {
	calls := 0
	handle := func(ctx context.Context, r Record) error {
		calls++
		return nil
	}
	ok := processRecord(context.Background(), Record{}, handle, nil, 3, testRetryDelay)
	if !ok {
		t.Error("processRecord() = false, want true")
	}
	if calls != 1 {
		t.Errorf("handle called %d times, want 1", calls)
	}
}

func TestProcessRecord_SucceedsAfterRetries(t *testing.T) {
	calls := 0
	handle := func(ctx context.Context, r Record) error {
		calls++
		if calls < 3 {
			return errors.New("transient failure")
		}
		return nil
	}
	ok := processRecord(context.Background(), Record{}, handle, nil, 3, testRetryDelay)
	if !ok {
		t.Error("processRecord() = false, want true")
	}
	if calls != 3 {
		t.Errorf("handle called %d times, want 3", calls)
	}
}

func TestProcessRecord_ExhaustedWithNilOnExhausted_StillCommits(t *testing.T) {
	calls := 0
	handle := func(ctx context.Context, r Record) error {
		calls++
		return errors.New("persistent failure")
	}
	ok := processRecord(context.Background(), Record{}, handle, nil, 3, testRetryDelay)
	if !ok {
		t.Error("processRecord() = false, want true (should still commit even when dropping)")
	}
	if calls != 3 {
		t.Errorf("handle called %d times, want 3 (handleAttempts)", calls)
	}
}

func TestProcessRecord_ExhaustedCallsOnExhausted(t *testing.T) {
	handle := func(ctx context.Context, r Record) error {
		return errors.New("persistent failure")
	}
	var gotRecord Record
	var gotErr error
	onExhausted := func(ctx context.Context, record Record, handleErr error) error {
		gotRecord = record
		gotErr = handleErr
		return nil
	}
	record := Record{Topic: "case-events", Partition: 2, Offset: 42}
	ok := processRecord(context.Background(), record, handle, onExhausted, 3, testRetryDelay)
	if !ok {
		t.Error("processRecord() = false, want true")
	}
	if gotRecord.Topic != "case-events" || gotRecord.Partition != 2 || gotRecord.Offset != 42 {
		t.Errorf("onExhausted got record = %+v, want the original record's identity preserved", gotRecord)
	}
	if !gotRecord.IsFinalAttempt {
		t.Error("onExhausted's record.IsFinalAttempt = false, want true")
	}
	if gotErr == nil || gotErr.Error() != "persistent failure" {
		t.Errorf("onExhausted handleErr = %v, want the last handle error", gotErr)
	}
}

func TestProcessRecord_OnExhaustedFailure_StillCommits(t *testing.T) {
	handle := func(ctx context.Context, r Record) error {
		return errors.New("persistent failure")
	}
	onExhausted := func(ctx context.Context, record Record, handleErr error) error {
		return errors.New("dead-letter topic unreachable")
	}
	ok := processRecord(context.Background(), Record{}, handle, onExhausted, 3, testRetryDelay)
	if !ok {
		t.Error("processRecord() = false, want true (nowhere lower to fall back to, so still commit)")
	}
}

// TestProcessRecord_OnExhaustedFailure_MakesCleanupCallWithNoMoreRetries is a
// regression test for a real gap CodeRabbit flagged: when the dead-letter
// publish itself fails, there is truly no future delivery of this record's
// content coming on any topic — but without an extra call, a Handle that
// tracks content-keyed idempotency state (see dispatch.Dispatcher) would
// never learn that and would leak its tracking forever. processRecord must
// call handle one more time, with NoMoreRetries set, purely so that
// cleanup can happen.
func TestProcessRecord_OnExhaustedFailure_MakesCleanupCallWithNoMoreRetries(t *testing.T) {
	var calls []Record
	handle := func(ctx context.Context, r Record) error {
		calls = append(calls, r)
		return errors.New("persistent failure")
	}
	onExhausted := func(ctx context.Context, record Record, handleErr error) error {
		return errors.New("dead-letter topic unreachable")
	}
	ok := processRecord(context.Background(), Record{}, handle, onExhausted, 3, testRetryDelay)
	if !ok {
		t.Error("processRecord() = false, want true")
	}
	if len(calls) != 4 {
		t.Fatalf("handle called %d times, want 4 (3 retries + 1 cleanup call after the dead-letter publish itself failed)", len(calls))
	}
	for i, c := range calls[:3] {
		if c.NoMoreRetries {
			t.Errorf("attempt %d: NoMoreRetries = true, want false (onExhausted hadn't been tried yet)", i+1)
		}
	}
	if !calls[3].NoMoreRetries {
		t.Error("cleanup call: NoMoreRetries = false, want true (the dead-letter publish failed — nothing will ever redeliver this content)")
	}
}

func TestProcessRecord_ContextCanceledMidRetry_DoesNotCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	handle := func(ctx context.Context, r Record) error {
		calls++
		if calls == 1 {
			cancel()
		}
		return errors.New("failure")
	}
	ok := processRecord(ctx, Record{}, handle, nil, 3, 50*time.Millisecond)
	if ok {
		t.Error("processRecord() = true, want false when ctx is canceled mid-retry-wait")
	}
	if calls != 1 {
		t.Errorf("handle called %d times, want 1 (should stop retrying once ctx is canceled)", calls)
	}
}

func TestProcessRecord_IsFinalAttemptOnlyOnLastCall(t *testing.T) {
	var finalFlags []bool
	handle := func(ctx context.Context, r Record) error {
		finalFlags = append(finalFlags, r.IsFinalAttempt)
		return errors.New("keep failing")
	}
	processRecord(context.Background(), Record{}, handle, nil, 3, testRetryDelay)
	want := []bool{false, false, true}
	if len(finalFlags) != len(want) {
		t.Fatalf("got %d attempts, want %d", len(finalFlags), len(want))
	}
	for i, w := range want {
		if finalFlags[i] != w {
			t.Errorf("attempt %d: IsFinalAttempt = %v, want %v", i+1, finalFlags[i], w)
		}
	}
}

// TestProcessRecord_NoMoreRetries_FalseWhenOnExhaustedSet verifies that a
// record with a dead-letter tier to fall back to (onExhausted != nil, the
// main topic's own Consumer.Run) never reports NoMoreRetries=true, even on
// its own final attempt — there's still a DLQ topic redelivery coming for
// the exact same content. See eventbus.Record.NoMoreRetries' doc comment
// and dispatch.recordBaseKey for why a Handle implementation that keys
// idempotency off content (not Kafka coordinates) depends on this.
func TestProcessRecord_NoMoreRetries_FalseWhenOnExhaustedSet(t *testing.T) {
	var flags []bool
	handle := func(ctx context.Context, r Record) error {
		flags = append(flags, r.NoMoreRetries)
		return errors.New("keep failing")
	}
	onExhausted := func(ctx context.Context, record Record, handleErr error) error { return nil }
	processRecord(context.Background(), Record{}, handle, onExhausted, 3, testRetryDelay)
	for i, got := range flags {
		if got {
			t.Errorf("attempt %d: NoMoreRetries = true, want false (onExhausted is set — a DLQ tier is still coming)", i+1)
		}
	}
}

// TestProcessRecord_NoMoreRetries_TrueOnFinalAttemptWhenNoOnExhausted
// verifies the DLQ topic's own Consumer.Run shape (onExhausted == nil):
// NoMoreRetries is false on every attempt except the last, where it's true
// — there really is nowhere further for this content to go.
func TestProcessRecord_NoMoreRetries_TrueOnFinalAttemptWhenNoOnExhausted(t *testing.T) {
	var flags []bool
	handle := func(ctx context.Context, r Record) error {
		flags = append(flags, r.NoMoreRetries)
		return errors.New("keep failing")
	}
	processRecord(context.Background(), Record{}, handle, nil, 3, testRetryDelay)
	want := []bool{false, false, true}
	if len(flags) != len(want) {
		t.Fatalf("got %d attempts, want %d", len(flags), len(want))
	}
	for i, w := range want {
		if flags[i] != w {
			t.Errorf("attempt %d: NoMoreRetries = %v, want %v", i+1, flags[i], w)
		}
	}
}
