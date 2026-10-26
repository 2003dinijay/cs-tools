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
	"reflect"
	"testing"
	"time"
)

// The CRE ladder counts its rungs from zero, as its rules sheet does; the SRE
// and SME ladders name each rung after the support tier it calls. One label
// for every ladder renamed the CRE first responders "L1" and its team lead
// "L2" on every case card.
func TestLevelLabelIn(t *testing.T) {
	cases := []struct {
		ladder Ladder
		want   []string
	}{
		{LadderCRE, []string{"L0", "L1", "L2", "L3", "L4"}},
		{LadderSRE, []string{"L1", "L2", "L3", "L4", "L5"}},
		{LadderSME, []string{"L1", "L2", "L3", "L4", "L5"}},
	}
	for _, c := range cases {
		var got []string
		for l := Level0; l <= Level4; l++ {
			got = append(got, l.LabelIn(c.ladder))
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ladder %q: labels %v, want %v", c.ladder, got, c.want)
		}
	}
	if got := Level(7).LabelIn(LadderCRE); got != "LEVEL_UNKNOWN(7)" {
		t.Errorf("out-of-range level = %q, want LEVEL_UNKNOWN(7)", got)
	}
}

// labelPlan is a two-rung plan on one ladder: LEVEL_0 now, LEVEL_1 five minutes on.
func labelPlan(ladder Ladder) Plan {
	at := time.Date(2026, 10, 21, 8, 46, 0, 0, time.UTC)
	return Plan{
		Trigger: Trigger{
			IncidentID: "case-1", Priority: "HIGH", At: at,
			Routing: RoutingContext{Ladder: ladder},
		},
		Calls: []PlannedCall{
			{Level: Level0, Ordinal: 1, At: at, Recipient: Recipient{Name: "First Responder"}},
			{Level: Level1, Ordinal: 1, At: at.Add(5 * time.Minute), Recipient: Recipient{Name: "Team Lead"}},
		},
	}
}

// The rung card, its "Escalates to" line and its header verb all follow the
// ladder: a CRE case card says "Notifying L0 ... Escalates to L1", an SRE one
// "Notifying L1 ... Escalates to L2". The verb is carried as FirstRung, not
// read off the label, so the CRE team lead (L1) is never "Notifying".
func TestChatCard_RungFollowsTheLadder(t *testing.T) {
	cases := []struct {
		ladder              Ladder
		first, next, second string
	}{
		{LadderCRE, "L0", "L1", "L1"},
		{LadderSRE, "L1", "L2", "L2"},
	}
	for _, c := range cases {
		chat := &fakeChat{}
		n := chatNotifier{chat: chat, links: fakeLinks{}, audience: "Room"}
		plan := labelPlan(c.ladder)
		for _, call := range plan.Calls {
			if _, err := n.Deliver(context.Background(), plan, call); err != nil {
				t.Fatalf("ladder %q: deliver %s: %v", c.ladder, call.Level, err)
			}
		}
		if len(chat.posted) != 2 {
			t.Fatalf("ladder %q: posted %d cards, want 2", c.ladder, len(chat.posted))
		}
		first, second := chat.posted[0], chat.posted[1]
		if first.Rung != c.first || !first.FirstRung {
			t.Errorf("ladder %q: first card rung = %q (firstRung %v), want %q and the notification level",
				c.ladder, first.Rung, first.FirstRung, c.first)
		}
		if first.NextRung != c.next || first.NextIn != "5m" {
			t.Errorf("ladder %q: first card escalates to %q in %q, want %q in 5m", c.ladder, first.NextRung, first.NextIn, c.next)
		}
		if second.Rung != c.second || second.FirstRung {
			t.Errorf("ladder %q: second card rung = %q (firstRung %v), want %q and an escalation",
				c.ladder, second.Rung, second.FirstRung, c.second)
		}
	}
}

// The stopped reply lists who each rung reached under the same label the
// rung's card used.
func TestStoppedReply_PagedListFollowsTheLadder(t *testing.T) {
	cases := []struct {
		ladder Ladder
		want   []string
	}{
		{LadderCRE, []string{"L0 First Responder", "L1 Team Lead"}},
		{LadderSRE, []string{"L1 First Responder", "L2 Team Lead"}},
	}
	for _, c := range cases {
		st := LadderState{Plan: labelPlan(c.ladder), Placed: []bool{true, true}}
		if got := pagedList(st); !reflect.DeepEqual(got, c.want) {
			t.Errorf("ladder %q: paged %v, want %v", c.ladder, got, c.want)
		}
	}
}
