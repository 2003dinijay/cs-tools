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

// Package suspensionstate translates between the real, confirmed shape of a
// project's suspensionProcessState (a free-form JSON blob written by an
// existing ServiceNow suspension flow, keyed by
// based_on_subscription_end_date/based_on_due_invoices/based_on_compliance —
// confirmed via a real write against the dedicated test project
// e3e87599-1bc7-6650-182c-0dc5604bcb68) and this component's own
// closure.NoticeWindow. Phase 1 only ever reads or writes the
// based_on_subscription_end_date key; based_on_due_invoices and
// based_on_compliance belong to Phase 2 and must survive every write
// untouched.
package suspensionstate

import (
	"encoding/json"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/acp-closure-service/internal/closure"
)

const subscriptionEndDateKey = "based_on_subscription_end_date"

// eventTypeToWindow maps the legacy event_type vocabulary observed in
// based_on_subscription_end_date to closure.NoticeWindow.
var eventTypeToWindow = map[string]closure.NoticeWindow{
	"90_days_notice": closure.NoticeWindow90,
	"60_days_notice": closure.NoticeWindow60,
	"30_days_notice": closure.NoticeWindow30,
	"15_days_notice": closure.NoticeWindow15,
	"7_days_notice":  closure.NoticeWindow7,
	"suspend":        closure.NoticeWindow0,
}

// windowToEventType is the inverse of eventTypeToWindow, used when writing a
// new based_on_subscription_end_date state back.
var windowToEventType = map[closure.NoticeWindow]string{
	closure.NoticeWindow90: "90_days_notice",
	closure.NoticeWindow60: "60_days_notice",
	closure.NoticeWindow30: "30_days_notice",
	closure.NoticeWindow15: "15_days_notice",
	closure.NoticeWindow7:  "7_days_notice",
	closure.NoticeWindow0:  "suspend",
}

type subscriptionEndDateState struct {
	EventType string `json:"event_type"`
}

// LastNoticeWindow extracts closure.NoticeWindow from the
// based_on_subscription_end_date.event_type field of a raw
// suspensionProcessState blob. Returns nil if the blob is empty, the key is
// absent, or event_type is "open" — meaning no prior notice has fired.
func LastNoticeWindow(raw json.RawMessage) (*closure.NoticeWindow, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var blob map[string]json.RawMessage
	if err := json.Unmarshal(raw, &blob); err != nil {
		return nil, fmt.Errorf("suspensionstate: parse blob: %w", err)
	}

	section, ok := blob[subscriptionEndDateKey]
	if !ok {
		return nil, nil
	}

	var state subscriptionEndDateState
	if err := json.Unmarshal(section, &state); err != nil {
		return nil, fmt.Errorf("suspensionstate: parse %s: %w", subscriptionEndDateKey, err)
	}

	window, ok := eventTypeToWindow[state.EventType]
	if !ok {
		return nil, nil
	}
	return &window, nil
}

// WithSubscriptionEndDateState returns a copy of raw with only the
// based_on_subscription_end_date key replaced by the given window and
// action results. based_on_due_invoices, based_on_compliance, and any other
// keys present in raw are preserved byte-for-byte — they are never
// unmarshaled into a typed structure and re-serialized, only carried through
// as raw JSON, so nothing about their formatting or content can drift.
func WithSubscriptionEndDateState(raw json.RawMessage, window closure.NoticeWindow, actions map[string]string) (json.RawMessage, error) {
	blob := map[string]json.RawMessage{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &blob); err != nil {
			return nil, fmt.Errorf("suspensionstate: parse blob: %w", err)
		}
	}

	section := map[string]string{"event_type": windowToEventType[window]}
	for action, result := range actions {
		section[action] = result
	}

	sectionRaw, err := json.Marshal(section)
	if err != nil {
		return nil, fmt.Errorf("suspensionstate: marshal %s: %w", subscriptionEndDateKey, err)
	}
	blob[subscriptionEndDateKey] = sectionRaw

	out, err := json.Marshal(blob)
	if err != nil {
		return nil, fmt.Errorf("suspensionstate: marshal blob: %w", err)
	}
	return out, nil
}
