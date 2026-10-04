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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

type fakeNotificationSweeper struct {
	OutageNotificationService
	calls int
	res   domain.OutageNotificationSweepResponse
	err   error
}

func (f *fakeNotificationSweeper) Sweep(context.Context, int) (domain.OutageNotificationSweepResponse, error) {
	f.calls++
	return f.res, f.err
}

type fakeCommunicationSweeper struct {
	OutageCommunicationService
	calls int
	res   domain.OutageCommunicationSweepResponse
	err   error
}

func (f *fakeCommunicationSweeper) Sweep(context.Context, int) (domain.OutageCommunicationSweepResponse, error) {
	f.calls++
	return f.res, f.err
}

type outagePublished struct {
	typ     events.Type
	id      string
	payload events.OutageNoticePayload
}

type fakeOutagePublisher struct {
	got    []outagePublished
	failID string // Publish fails for this entity id
}

func (f *fakeOutagePublisher) Publish(_ context.Context, t events.Type, id string, raw json.RawMessage) error {
	if id == f.failID {
		return errors.New("broker unavailable")
	}
	var p events.OutageNoticePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	f.got = append(f.got, outagePublished{t, id, p})
	return nil
}

func TestOutageNoticeDrainerPublishesEachDecisionToItsAudience(t *testing.T) {
	notif := &fakeNotificationSweeper{res: domain.OutageNotificationSweepResponse{Decisions: []domain.OutageNotificationDecision{
		{OutageID: "o1", Number: "OUT0010001", Kind: domain.OutageNotificationDeclared, Subject: "[Outage] OUT0010001 - Declared", Body: "Outage OUT0010001 declared."},
	}}}
	comm := &fakeCommunicationSweeper{res: domain.OutageCommunicationSweepResponse{Decisions: []domain.OutageCommunicationDecision{
		{OutageID: "o1", Number: "OUT0010001", Kind: domain.OutageCommunicationDeclared, Subject: "Choreo Outage OUT0010001 on x", Body: "Hello Team,"},
	}}}
	pub := &fakeOutagePublisher{}
	d := &OutageNoticeDrainer{
		Notifications: notif, Communications: comm, Publisher: pub,
		NotificationRecipients:  []string{"stakeholders@wso2.com"},
		CommunicationRecipients: []string{"sre@wso2.com"},
	}

	if n := d.drainOnce(context.Background()); n != 2 {
		t.Fatalf("outagePublished %d, want 2", n)
	}
	if len(pub.got) != 2 {
		t.Fatalf("got %d events, want 2", len(pub.got))
	}
	first, second := pub.got[0], pub.got[1]
	if first.typ != events.TypeOutageNotificationDue || first.id != "o1" ||
		first.payload.Kind != "DECLARED" || first.payload.Recipients[0] != "stakeholders@wso2.com" ||
		first.payload.Subject != "[Outage] OUT0010001 - Declared" {
		t.Errorf("internal notification event: %+v", first)
	}
	if second.typ != events.TypeOutageCommunicationDue || second.payload.Recipients[0] != "sre@wso2.com" ||
		second.payload.Body != "Hello Team," {
		t.Errorf("outage communication event: %+v", second)
	}
}

// A sweep records what it decides, so a flow nobody receives must not be swept
// at all -- otherwise switching its recipients on later would find every email
// already "sent".
func TestOutageNoticeDrainerDoesNotSweepAFlowWithNoRecipients(t *testing.T) {
	notif := &fakeNotificationSweeper{}
	comm := &fakeCommunicationSweeper{}
	d := &OutageNoticeDrainer{Notifications: notif, Communications: comm, Publisher: &fakeOutagePublisher{},
		CommunicationRecipients: []string{"sre@wso2.com"}}

	d.drainOnce(context.Background())

	if notif.calls != 0 {
		t.Errorf("internal notification swept %d times with no recipients, want 0", notif.calls)
	}
	if comm.calls != 1 {
		t.Errorf("outage communication swept %d times, want 1", comm.calls)
	}
}

func TestOutageNoticeDrainerKeepsGoingPastFailures(t *testing.T) {
	notif := &fakeNotificationSweeper{err: errors.New("database down")}
	comm := &fakeCommunicationSweeper{res: domain.OutageCommunicationSweepResponse{Decisions: []domain.OutageCommunicationDecision{
		{OutageID: "bad", Number: "OUT1", Kind: domain.OutageCommunicationResolved},
		{OutageID: "good", Number: "OUT2", Kind: domain.OutageCommunicationResolved},
	}}}
	pub := &fakeOutagePublisher{failID: "bad"}
	d := &OutageNoticeDrainer{Notifications: notif, Communications: comm, Publisher: pub,
		NotificationRecipients: []string{"a@wso2.com"}, CommunicationRecipients: []string{"b@wso2.com"}}

	if n := d.drainOnce(context.Background()); n != 1 {
		t.Fatalf("outagePublished %d, want 1 (the failed sweep and the failed publish must not stop the rest)", n)
	}
	if pub.got[0].id != "good" {
		t.Errorf("outagePublished %q, want the decision after the failed one", pub.got[0].id)
	}
}
