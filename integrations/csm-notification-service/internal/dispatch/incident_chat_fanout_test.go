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

package dispatch

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/notifications"
)

// INCIDENT_CHAT_FANOUT: a default group's incident cards (the incident of a
// service mapped to no SRE team) go to both Apollo's and Artemis's spaces.

const defaultGroupIncidentCreated = `{"type":"incident.created","entityId":"inc-1","payload":{"title":"ContainerRecentlyOOMKilled","shortDescription":"ContainerRecentlyOOMKilled","callTo":"+15551234567","number":"INC0088718","priority":"CRITICAL","team":"Default SRE Group","category":"SERVICE_INTERRUPTION","state":"NEW","service":"wso2-cloud","environment":"central"}}`

const defaultGroupIncidentAssigned = `{"type":"incident.assigned","entityId":"inc-1","payload":{"assigneeId":"u-1","assigneeName":"Shan Anjana","number":"INC0088718","team":"Default SRE Group","updatedBy":"shana@wso2.com"}}`

var (
	apolloSpace          = notifications.IncidentAudience("Apollo SRE Group")
	artemisSpaceAudience = notifications.IncidentAudience("Artemis SRE Group")
	bothSaaSFanout       = map[string][]string{"Default SRE Group": {"Apollo SRE Group", "Artemis SRE Group"}}
)

// saasSpaces is INCIDENT_CHAT_SPACES with Apollo and Artemis configured.
func saasSpaces(audience string) bool {
	return audience == apolloSpace || audience == artemisSpaceAudience
}

// failingChat is mockGoogleChatSender whose incident cards fail for some audiences.
type failingChat struct {
	*mockGoogleChatSender
	failFor map[string]bool
}

func (f failingChat) SendIncidentCreatedAlert(ctx context.Context, audience string, a notifications.IncidentCreatedAlert) error {
	_ = f.mockGoogleChatSender.SendIncidentCreatedAlert(ctx, audience, a)
	if f.failFor[audience] {
		return errors.New("chat down")
	}
	return nil
}

func createdAudiences(m *mockGoogleChatSender) []string {
	var out []string
	for _, c := range m.incidentCreatedCalls {
		out = append(out, c.audience)
	}
	return out
}

func TestDispatcher_IncidentChatFanout_ADefaultGroupPostsToApolloAndArtemis(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: saasSpaces}
	call := &mockCallSender{}
	d := NewDispatcher(&mockEmailSender{}, chat, call, &mockLinkResolver{}, true, false, nil, true, "", nil).
		WithIncidentChatFanout(bothSaaSFanout)

	if err := d.Handle(context.Background(), eventbus.Record{Value: []byte(defaultGroupIncidentCreated)}); err != nil {
		t.Fatalf("Handle(created) error = %v", err)
	}
	if got, want := createdAudiences(chat), []string{apolloSpace, artemisSpaceAudience}; !slices.Equal(got, want) {
		t.Errorf("incident cards went to %v; want %v", got, want)
	}
	if chat.incidentCreatedCalls[0].alert.Team != "Default SRE Group" {
		t.Errorf("card team = %q; want the incident's own group", chat.incidentCreatedCalls[0].alert.Team)
	}
	if len(call.calls) != 1 {
		t.Errorf("calls = %d; want 1, whatever the number of spaces", len(call.calls))
	}

	if err := d.Handle(context.Background(), eventbus.Record{Value: []byte(defaultGroupIncidentAssigned)}); err != nil {
		t.Fatalf("Handle(assigned) error = %v", err)
	}
	var replies []string
	for _, c := range chat.incidentAssignedCalls {
		replies = append(replies, c.audience)
	}
	if want := []string{apolloSpace, artemisSpaceAudience}; !slices.Equal(replies, want) {
		t.Errorf("Incident Acknowledged replies went to %v; want %v", replies, want)
	}
}

// A group INCIDENT_CHAT_FANOUT does not name keeps its own space, and with
// no fan-out configured a default group with no space gets no card, as before.
func TestDispatcher_IncidentChatFanout_OtherGroupsUnchanged(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: saasSpaces}
	d := NewDispatcher(&mockEmailSender{}, chat, &mockCallSender{}, &mockLinkResolver{}, true, false, nil, true, "", nil).
		WithIncidentChatFanout(bothSaaSFanout)
	if err := d.Handle(context.Background(), eventbus.Record{Value: []byte(incidentCreatedWithTeam)}); err != nil {
		t.Fatal(err)
	}
	if got := createdAudiences(chat); !slices.Equal(got, []string{artemisSpaceAudience}) {
		t.Errorf("an Artemis incident's cards went to %v; want Artemis's space only", got)
	}

	unset := &mockGoogleChatSender{hasAudienceSpace: saasSpaces}
	d = NewDispatcher(&mockEmailSender{}, unset, &mockCallSender{}, &mockLinkResolver{}, true, false, nil, true, "", nil)
	if err := d.Handle(context.Background(), eventbus.Record{Value: []byte(defaultGroupIncidentCreated)}); err != nil {
		t.Fatal(err)
	}
	if len(unset.incidentCreatedCalls) != 0 {
		t.Errorf("without INCIDENT_CHAT_FANOUT a default group with no space posted %d card(s); want none", len(unset.incidentCreatedCalls))
	}
}

// A listed group with no INCIDENT_CHAT_SPACES entry is skipped; the others still get the card.
func TestDispatcher_IncidentChatFanout_SkipsAGroupWithNoSpace(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: func(a string) bool { return a == artemisSpaceAudience }}
	d := NewDispatcher(&mockEmailSender{}, chat, &mockCallSender{}, &mockLinkResolver{}, true, false, nil, true, "", nil).
		WithIncidentChatFanout(bothSaaSFanout)
	if err := d.Handle(context.Background(), eventbus.Record{Value: []byte(defaultGroupIncidentCreated)}); err != nil {
		t.Fatal(err)
	}
	if got := createdAudiences(chat); !slices.Equal(got, []string{artemisSpaceAudience}) {
		t.Errorf("cards went to %v; want Artemis's space only", got)
	}
}

// One space failing while the other took the card is not a failed record (a
// retry would post it twice where it landed); a card no space took is retried.
func TestDispatcher_IncidentChatFanout_Failures(t *testing.T) {
	record := eventbus.Record{Topic: "case-events", Partition: 1, Offset: 9, Value: []byte(defaultGroupIncidentCreated)}

	partial := failingChat{mockGoogleChatSender: &mockGoogleChatSender{hasAudienceSpace: saasSpaces}, failFor: map[string]bool{apolloSpace: true}}
	d := NewDispatcher(&mockEmailSender{}, partial, &mockCallSender{}, &mockLinkResolver{}, true, false, nil, true, "", nil).
		WithIncidentChatFanout(bothSaaSFanout)
	if err := d.Handle(context.Background(), record); err != nil {
		t.Errorf("Apollo down, Artemis took it: Handle() error = %v; want delivered", err)
	}

	down := failingChat{mockGoogleChatSender: &mockGoogleChatSender{hasAudienceSpace: saasSpaces}, failFor: map[string]bool{apolloSpace: true, artemisSpaceAudience: true}}
	d = NewDispatcher(&mockEmailSender{}, down, &mockCallSender{}, &mockLinkResolver{}, true, false, nil, true, "", nil).
		WithIncidentChatFanout(bothSaaSFanout)
	if err := d.Handle(context.Background(), record); err == nil {
		t.Error("both spaces down: Handle() error = nil; want an error, so the record retries")
	}
}

func TestDispatcher_IncidentChatFanout_KeysIgnoreCaseAndSpace(t *testing.T) {
	chat := &mockGoogleChatSender{hasAudienceSpace: saasSpaces}
	d := NewDispatcher(&mockEmailSender{}, chat, &mockCallSender{}, &mockLinkResolver{}, true, false, nil, true, "", nil).
		WithIncidentChatFanout(map[string][]string{"  default sre GROUP ": {"Apollo SRE Group", "apollo sre group", "Artemis SRE Group"}, "": {"Apollo SRE Group"}})
	if err := d.Handle(context.Background(), eventbus.Record{Value: []byte(defaultGroupIncidentCreated)}); err != nil {
		t.Fatal(err)
	}
	if got, want := createdAudiences(chat), []string{apolloSpace, artemisSpaceAudience}; !slices.Equal(got, want) {
		t.Errorf("cards went to %v; want %v, each space once", got, want)
	}
}
