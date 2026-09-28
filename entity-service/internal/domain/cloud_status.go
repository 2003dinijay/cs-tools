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

package domain

// Cloud status webhooks
//
// The port of ServiceNow's `Cloud Status Event Notification Flow`. When an
// outage against one of the monitored cloud services begins or ends, the
// public status dashboard is told over a webhook.
//
// Only the webhook half is ported here. The flow also rewrote cloud monitor
// status for every affected configuration item; a separate service owns that,
// so nothing in this package writes cloud_monitor.

// CloudStatusEvent is the `event` string the webhook body carries.
type CloudStatusEvent string

const (
	// CloudStatusEventOutageBegin is confirmed: the flow's step 15 passes this
	// exact literal.
	CloudStatusEventOutageBegin CloudStatusEvent = "OUTAGE_BEGIN"

	// CloudStatusEventOutageEnd is the completed arm, step 8.
	//
	// *** THE WIRE VALUE FOR THIS ONE IS NOT CONFIRMED. *** Step 8's Event
	// input was never captured from ServiceNow -- only step 15's. The name
	// here is this service's own and is safe; what is a guess is
	// CloudStatusEventWireValue's mapping of it to "outage_end". The
	// dashboard switches on that string, so a wrong guess is a silent
	// no-op on the receiving side, not an error anyone sees. Confirm it
	// against the flow before this goes anywhere near production.
	CloudStatusEventOutageEnd CloudStatusEvent = "OUTAGE_END"
)

// cloudStatusWireValues maps this service's event names to the literals
// ServiceNow puts on the wire. Kept separate from the constants above so the
// storage enum can be renamed freely while the wire contract -- which belongs
// to the dashboard, not to us -- stays pinned.
var cloudStatusWireValues = map[CloudStatusEvent]string{
	CloudStatusEventOutageBegin: "outage_begin",
	CloudStatusEventOutageEnd:   "outage_end", // UNCONFIRMED -- see above.
}

// WireValue returns the literal to send in the webhook body.
func (e CloudStatusEvent) WireValue() string {
	if v, ok := cloudStatusWireValues[e]; ok {
		return v
	}
	return ""
}

// Valid reports whether e is a known event.
func (e CloudStatusEvent) Valid() bool {
	_, ok := cloudStatusWireValues[e]
	return ok
}

// cloudOfferingWireValues maps cloud_monitor.cloud_offering -- a Postgres
// enum, so SCREAMING_SNAKE -- to the slug the status dashboard is addressed
// by. The dashboard routes on this string, and CHOREO_EU -> "choreo-eu" is
// exactly the kind of transformation that silently produces "choreo_eu" and a
// webhook nobody receives.
//
// ALL SEVEN OFFERINGS ARE HANDLED HERE, and that is a deliberate divergence.
// ServiceNow's step-14 script had a five-way lookup -- asgardeo, bijira,
// choreo, devant, moesif -- and returned undefined for choreo-eu and
// agent-manager, posting to a URL of "undefined" with no error raised.
//
// That defect is latent rather than live in ServiceNow: all 31 monitors on
// those two offerings sit outside the 14 services the trigger filters to, so
// the flow never reaches them. It goes live the moment anyone adds a
// choreo-eu or agent-manager service to that filter -- a config change, made
// by someone who has no reason to suspect a hardcoded list exists. Covering
// all seven costs nothing and removes the trap rather than porting it.
var cloudOfferingWireValues = map[string]string{
	"ASGARDEO":      "asgardeo",
	"BIJIRA":        "bijira",
	"CHOREO":        "choreo",
	"DEVANT":        "devant",
	"MOESIF":        "moesif",
	"CHOREO_EU":     "choreo-eu",     // ServiceNow had no URL for this one.
	"AGENT_MANAGER": "agent-manager", // nor this one.
}

// CloudOfferingSlug converts a stored cloud_offering enum value to the
// dashboard's slug. It returns "" for an unknown value, which callers must
// treat as unroutable rather than posting an empty cloud.
func CloudOfferingSlug(offering string) string {
	return cloudOfferingWireValues[offering]
}

// PendingCloudStatusWebhook is one webhook this service has decided is owed to
// the dashboard and not yet seen delivered.
//
// The cloud is resolved once, when the row is recorded, and carried here as
// stored rather than re-derived on each attempt: an outage's configuration
// item can be corrected after the fact, and a retry must repeat the original
// post rather than quietly become a different one.
type PendingCloudStatusWebhook struct {
	ID       string `json:"id"`
	OutageID string `json:"outageId"`
	// Number is carried for logging and for the operator reading a failure.
	// It is not part of the webhook body.
	Number string           `json:"number"`
	Event  CloudStatusEvent `json:"event"`
	Cloud  string           `json:"cloud"`
	// Timestamp is the outage's own begin or end instant -- whichever this
	// event is about -- not the moment of sending.
	//
	// ServiceNow sent `new Date()` here, i.e. send-time. That is a divergence
	// on paper and a fidelity IMPROVEMENT in practice: the flow fired
	// synchronously on the record update, so its send-time was within a second
	// of the transition and the two were interchangeable. This port decides on
	// a sweep and delivers on a later tick, so send-time could be minutes off,
	// and a retried webhook hours. Sending the instant the outage actually
	// changed keeps the dashboard agreeing with what ServiceNow DID, rather
	// than with what its code literally said.
	Timestamp    string `json:"timestamp"`
	AttemptCount int    `json:"attemptCount"`
	LastError    string `json:"lastError,omitempty"`
}

// PendingCloudStatusWebhooksResponse is the body of the pending-webhook read.
type PendingCloudStatusWebhooksResponse struct {
	Count    int                         `json:"count"`
	Webhooks []PendingCloudStatusWebhook `json:"webhooks"`
}

// CloudStatusSweepResponse reports what one decision sweep recorded.
type CloudStatusSweepResponse struct {
	// Scanned is how many in-scope outages the sweep considered.
	Scanned int `json:"scanned"`
	// Recorded is how many new transitions it owed the dashboard. Zero is the
	// steady state; a sweep that records nothing is working correctly.
	Recorded int `json:"recorded"`
	// SkippedNoCloud counts in-scope outages whose configuration item has no
	// cloud monitor, and which therefore cannot be routed. ServiceNow's flow
	// failed its whole execution on this; this port skips the outage and keeps
	// going, so the count is the only way to notice.
	SkippedNoCloud int `json:"skippedNoCloud"`
}

// RecordCloudStatusDeliveryRequest reports the outcome of one webhook attempt.
type RecordCloudStatusDeliveryRequest struct {
	ID        string `json:"-"`
	Delivered bool   `json:"delivered"`
	Error     string `json:"error,omitempty"`
}
