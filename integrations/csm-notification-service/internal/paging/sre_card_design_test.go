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

package paging

import (
	"context"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// orderedChat records the order of the incident card and the rung posts.
type orderedChat struct {
	*fakeChat
	order []string
}

func (o *orderedChat) SendIncidentCreatedAlert(ctx context.Context, audience string, a notifications.IncidentCreatedAlert) error {
	o.order = append(o.order, "incident")
	return o.fakeChat.SendIncidentCreatedAlert(ctx, audience, a)
}

func (o *orderedChat) SendEscalationAlert(ctx context.Context, a notifications.EscalationAlert) error {
	o.order = append(o.order, a.Rung)
	return o.fakeChat.SendEscalationAlert(ctx, a)
}

// rungFails takes the incident card and rejects every rung.
type rungFails struct{ *fakeChat }

func (rungFails) SendEscalationAlert(context.Context, notifications.EscalationAlert) error {
	return errors.New("webhook down")
}

// An SRE incident's space gets one "Incident Reported" card, then every rung and the closing message as replies under it.
func TestSRECards_IncidentCardThenReplies(t *testing.T) {
	shared, store := &fakeChat{}, newMemStore()
	chat := &orderedChat{fakeChat: shared}
	e := sreClosingEngine(shared, store, nil)
	e.notifiers = []notifier{chatNotifier{chat: chat, links: fakeLinks{}, audience: "SRE"}}

	runUnanswered(t, e, "Apollo")

	if len(shared.incidents) != 1 {
		t.Fatalf("incident cards = %d; want 1", len(shared.incidents))
	}
	if got := chat.order; len(got) != 4 || got[0] != "incident" || got[1] != "L1" {
		t.Errorf("post order = %v; want the incident card before the L1 reply", got)
	}
	card := shared.incidents[0]
	if card.IncidentID != testIncidentID || card.Number != "INC0099001" || card.Team != "Apollo" || card.Priority != "HIGH" ||
		card.ShortDescription != "Latency alert on gateway" || card.IncidentLink != (fakeLinks{}).IncidentLink(testIncidentID) {
		t.Errorf("incident card = %+v", card)
	}
	if shared.incidentAudiences[0] != "SRE" {
		t.Errorf("incident card went to %q; want the ladder's space", shared.incidentAudiences[0])
	}
	for _, a := range shared.posted {
		if !a.Reply || a.ThreadKey != notifications.IncidentThreadKey(testIncidentID) {
			t.Errorf("rung %s reply=%v thread=%q; want a reply in the incident card's thread", a.Rung, a.Reply, a.ThreadKey)
		}
	}
	if len(shared.unanswered) != 1 || !shared.unanswered[0].Reply {
		t.Errorf("closing messages = %+v; want one reply", shared.unanswered)
	}
}

// An assigned SRE incident ends its thread with the "Incident Acknowledged." card, naming only the assignee.
func TestSRECards_AssignedPostsIncidentAcknowledged(t *testing.T) {
	chat, store := &fakeChat{}, newMemStore()
	e := ladderEngine(LadderSRE, chat, store)

	stoppedRun(t, e, store, created(t, "Apollo", "HIGH"),
		record(t, events.TypeIncidentAssigned, events.IncidentAssignedPayload{AssigneeID: "a-l2", AssigneeName: "Jane Doe", UpdatedBy: "John Smith"}))

	if len(chat.incidents) != 1 || len(chat.stopped) != 0 || len(chat.acks) != 1 {
		t.Fatalf("incident cards %d, stopped %d, acknowledged %d; want one card and one acknowledged reply", len(chat.incidents), len(chat.stopped), len(chat.acks))
	}
	a := chat.acks[0]
	if a.IncidentID != testIncidentID || a.Number != "INC0099001" || a.AssigneeName != "Jane Doe" || a.UpdatedBy != "" || a.IncidentLink != "" {
		t.Errorf("acknowledged = %+v", a)
	}
}

// Any other SRE stop keeps the stopped reply under the card.
func TestSRECards_OtherStopsAreStoppedReplies(t *testing.T) {
	chat, store := &fakeChat{}, newMemStore()
	e := ladderEngine(LadderSRE, chat, store)

	stoppedRun(t, e, store, created(t, "Apollo", "HIGH"),
		record(t, events.TypeIncidentAcknowledged, events.IncidentAcknowledgedPayload{PreviousState: "NEW", NewState: "IN_PROGRESS"}))

	if len(chat.acks) != 0 || len(chat.stopped) != 1 || !chat.stopped[0].Reply {
		t.Errorf("acknowledged %d, stopped %+v; want one stopped reply", len(chat.acks), chat.stopped)
	}
}

// A CRE case keeps the full cards: no incident card, no replies.
func TestSRECards_CREUnchanged(t *testing.T) {
	chat, store := &fakeChat{}, newMemStore()
	e := ladderEngine(LadderCRE, chat, store)

	stoppedRun(t, e, store, caseCreated(t, "Atlas", "HIGH"),
		caseAssigned(t, "eng@wso2.com"),
		caseComment(t, "eng@wso2.com", false, true))

	if len(chat.incidents) != 0 {
		t.Errorf("incident cards = %d; a CRE ladder posts none", len(chat.incidents))
	}
	if len(chat.posted) == 0 || chat.posted[0].Reply {
		t.Errorf("rung cards = %+v; want the full card", chat.posted)
	}
	if len(chat.stopped) != 1 || chat.stopped[0].Reply {
		t.Errorf("stopped = %+v; want the full card", chat.stopped)
	}
}

// An S0 case on the SRE ladder has no incident card to reply under, so it keeps the full cards.
func TestSRECards_SRECaseUnchanged(t *testing.T) {
	chat, store := &fakeChat{}, newMemStore()
	e := ladderEngine(LadderSRE, chat, store)

	stoppedRun(t, e, store, caseCreated(t, "Atlas", "CATASTROPHIC"))

	if len(chat.incidents) != 0 || len(chat.posted) == 0 || chat.posted[0].Reply {
		t.Errorf("incident cards %d, rungs %+v; want the full rung card and no incident card", len(chat.incidents), chat.posted)
	}
}

// An INCIDENT_CHAT_SPACES space already has the dispatcher's card, so the ladder posts only replies there.
func TestSRECards_IncidentSpaceGetsNoSecondCard(t *testing.T) {
	chat := &fakeChat{}
	n := chatNotifier{chat: chat, links: fakeLinks{}, audience: notifications.IncidentAudience("Apollo"), reply: true}
	plan := Plan{
		Trigger: Trigger{IncidentID: "inc-1", Number: "INC1", Record: RecordIncident, Routing: RoutingContext{Ladder: LadderSRE}},
		Calls:   []PlannedCall{{Level: Level0, Ordinal: 1, Recipient: Recipient{Name: "Jane"}}},
	}

	if _, err := n.Deliver(context.Background(), plan, plan.Calls[0]); err != nil {
		t.Fatal(err)
	}
	if len(chat.incidents) != 0 || len(chat.posted) != 1 || !chat.posted[0].Reply {
		t.Errorf("incident cards %d, rungs %+v; want one reply and no card", len(chat.incidents), chat.posted)
	}
}

// Only the plan's first call posts the incident card; a later rung is a reply alone.
func TestSRECards_OnlyTheFirstCallPostsTheCard(t *testing.T) {
	chat := &fakeChat{}
	n := chatNotifier{chat: chat, links: fakeLinks{}, audience: "SRE"}
	plan := Plan{
		Trigger: Trigger{IncidentID: "inc-1", Number: "INC1", Record: RecordIncident, Routing: RoutingContext{Ladder: LadderSRE}},
		Calls: []PlannedCall{
			{Level: Level0, Ordinal: 1, Recipient: Recipient{Name: "Jane"}},
			{Level: Level1, Ordinal: 1, Recipient: Recipient{Name: "Lead"}},
		},
	}

	if _, err := n.Deliver(context.Background(), plan, plan.Calls[1]); err != nil {
		t.Fatal(err)
	}
	if len(chat.incidents) != 0 || len(chat.posted) != 1 || !chat.posted[0].Reply {
		t.Errorf("incident cards %d, rungs %+v; want one reply and no card", len(chat.incidents), chat.posted)
	}
}

// A failed incident card fails the rung, so the retry posts both; a failed first reply after the card is not retried, so the card is never posted twice.
func TestSRECards_FailuresNeverDuplicateTheCard(t *testing.T) {
	plan := Plan{
		Trigger: Trigger{IncidentID: "inc-1", Number: "INC1", Record: RecordIncident, Routing: RoutingContext{Ladder: LadderSRE}},
		Calls:   []PlannedCall{{Level: Level0, Ordinal: 1, Recipient: Recipient{Name: "Jane"}}},
	}
	ctx := context.Background()

	down := &fakeChat{err: errors.New("webhook down")}
	if _, err := (chatNotifier{chat: down, audience: "SRE"}).Deliver(ctx, plan, plan.Calls[0]); err == nil {
		t.Error("a failed incident card should fail the rung so it is retried")
	}

	half := rungFails{&fakeChat{}}
	d, err := (chatNotifier{chat: half, audience: "SRE"}).Deliver(ctx, plan, plan.Calls[0])
	if err != nil {
		t.Fatalf("a failed reply after the card was posted must not be retried: %v", err)
	}
	if len(half.incidents) != 1 || d.Status == "" {
		t.Errorf("incident cards %d status %q; want the card posted once and the miss in the status", len(half.incidents), d.Status)
	}
}

// A call-only SRE ladder posts no rung cards, so no incident card exists and its closing, handoff and stopped messages stay full cards.
func TestSRECards_RepliesOnlyWhenTheLadderPostsCards(t *testing.T) {
	sre := Trigger{Team: "Apollo", Record: RecordIncident, Routing: RoutingContext{Ladder: LadderSRE}}
	for _, tc := range []struct {
		channel Channel
		trigger Trigger
		want    bool
	}{
		{ChannelChat, sre, true},
		{ChannelBoth, sre, true},
		{ChannelCall, sre, false},
		{ChannelLog, sre, false},
		{ChannelChat, Trigger{Record: RecordCase, Routing: RoutingContext{Ladder: LadderSRE}}, false},
		{ChannelChat, Trigger{Record: RecordIncident, Routing: RoutingContext{Ladder: LadderCRE}}, false},
		{ChannelChat, Trigger{Record: RecordIncident, Routing: RoutingContext{Ladder: LadderSME}}, false},
	} {
		e := &Engine{cfg: EngineConfig{Channel: tc.channel}}
		if got := e.sreReplies(tc.trigger); got != tc.want {
			t.Errorf("channel %s, ladder %q: sreReplies = %v; want %v", tc.channel, tc.trigger.Routing.Ladder, got, tc.want)
		}
	}
}
