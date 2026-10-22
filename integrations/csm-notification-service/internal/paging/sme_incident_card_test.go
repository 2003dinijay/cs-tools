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
	"testing"
	"time"
)

// An SME space gets one "Incident Escalated" card naming who escalated it and the L1 on call, then every rung as a reply under it.
func TestSMECards_IncidentCardThenReplies(t *testing.T) {
	ctx := context.Background()
	smeChat := &fakeChat{}
	e := smeEngine(smeTieredReader(), newMemStore(), &fakeNotes{}, &fakeChat{}, smeChat, smeOn)

	p := handoff("Apollo", "asgardeo", "asgardeo", testClock)
	p.Priority, p.ServiceName, p.State = "P1", "Gateway", "IN_PROGRESS"
	raise(t, e, p)
	for _, at := range []time.Duration{5 * time.Minute, 10 * time.Minute} {
		if err := e.Tick(ctx, testClock.Add(at)); err != nil {
			t.Fatal(err)
		}
	}

	if len(smeChat.smeIncidents) != 1 {
		t.Fatalf("SME incident cards = %d; want exactly one", len(smeChat.smeIncidents))
	}
	card := smeChat.smeIncidents[0]
	if card.EscalatedBy != "lead@example.com" || card.EscalatedFrom != "Apollo" {
		t.Errorf("escalated by %q from %q; want lead@example.com from Apollo", card.EscalatedBy, card.EscalatedFrom)
	}
	if card.OnCall != "s-a" || card.SMETeam != "Asgardeo" || card.Priority != "P1" || card.Service != "Gateway" || card.State != "IN_PROGRESS" {
		t.Errorf("card = %+v; want L1 s-a on call for Asgardeo, P1, Gateway, IN_PROGRESS", card)
	}
	if len(smeChat.incidents) != 0 {
		t.Errorf("an SME space got %d SRE \"Incident Reported\" cards; want none", len(smeChat.incidents))
	}
	if len(smeChat.posted) != 3 {
		t.Fatalf("SME rungs = %d; want L1, L2 and L3", len(smeChat.posted))
	}
	for _, r := range smeChat.posted {
		if !r.Reply || r.ThreadKey == "" {
			t.Errorf("rung %s: reply %v, thread %q; want a reply in the incident card's thread", r.Rung, r.Reply, r.ThreadKey)
		}
	}
}
