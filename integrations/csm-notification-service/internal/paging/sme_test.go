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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

// smeOn is the SME page on the chat channel, with the Special Ops team
// "choreo" mapped to its SME team.
var smeOn = SMEConfig{Enabled: true, Channel: ChannelChat, Teams: map[string]string{"Choreo": "choreo-sme"}}

// smeEngine is a chat-only SRE engine over reader, its ladder cards going to
// ladderChat and its SME pages to smeChat.
func smeEngine(reader *stubScheduleReader, store *memStore, notes *fakeNotes, ladderChat, smeChat *fakeChat, sme SMEConfig) *Engine {
	cfg := EngineConfig{CallSendingEnabled: true, Channel: ChannelChat, Kind: LadderSRE, SME: sme}
	return &Engine{
		policies:     withSREPolicy(DefaultPolicy, cfg.Ladder.Timing.Policy()),
		resolver:     rotaResolver(reader, "").WithCallHistory(store),
		notifiers:    []notifier{chatNotifier{chat: ladderChat, links: fakeLinks{}}},
		smeNotifiers: []notifier{chatNotifier{chat: smeChat, links: fakeLinks{}, audience: "Special Ops"}},
		store:        store,
		notes:        notes,
		cfg:          cfg,
		clock:        func() time.Time { return testClock },
	}
}

// handoff is one move of the incident into a Special Ops group, as
// entity-service publishes it: previousGroup is the group it left, smeTeam
// the publisher's SME team (may be empty), teamKey the Special Ops team.
func handoff(previousGroup, smeTeam, teamKey string, at time.Time) events.IncidentSpecialOpsAlertPayload {
	return events.IncidentSpecialOpsAlertPayload{
		IncidentID: testIncidentID, Number: "INC0099001", Subject: "Latency alert on gateway", Product: "Asgardeo",
		TeamKey: teamKey, TeamLabel: strings.ToUpper(teamKey[:1]) + teamKey[1:],
		AssignmentGroupID: "g-special-ops", AssignmentGroupName: "Special Ops",
		PreviousAssignmentGroupID: "g-prev", PreviousAssignmentGroupName: previousGroup,
		ChangedBy: "lead@example.com", ChangedOn: at.Format(time.RFC3339), SMETeam: smeTeam,
	}
}

// alertRecord is the same alert as a Kafka record.
func alertRecord(t *testing.T, p events.IncidentSpecialOpsAlertPayload) eventbus.Record {
	return record(t, events.TypeIncidentSpecialOpsAlert, p)
}

// raise delivers an alert the way the dispatcher's sre-events consumer does.
func raise(t *testing.T, e *Engine, p events.IncidentSpecialOpsAlertPayload) {
	t.Helper()
	if err := e.HandleSpecialOpsAlert(context.Background(), p.IncidentID, p); err != nil {
		t.Fatal(err)
	}
}

func handle(t *testing.T, e *Engine, rec eventbus.Record) {
	t.Helper()
	if err := e.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
}

// notesWith returns the work notes containing s.
func notesWith(notes *fakeNotes, s string) []string {
	var out []string
	for _, n := range notes.notes {
		if strings.Contains(n, s) {
			out = append(out, n)
		}
	}
	return out
}

// The press on a SaaS SRE incident: its chain stops, with the usual summary,
// and ONE SME on the chosen team's current window is paged.
func TestSME_StopsTheSaaSChainAndPagesOnePerson(t *testing.T) {
	ctx := context.Background()
	ladderChat, smeChat, store, notes := &fakeChat{}, &fakeChat{}, newMemStore(), &fakeNotes{}
	e := smeEngine(rotaReader(true), store, notes, ladderChat, smeChat, smeOn)

	handle(t, e, created(t, "Apollo", "HIGH"))
	if _, found, _ := store.Get(ctx, testIncidentID); !found {
		t.Fatal("no SRE ladder to stop")
	}
	raise(t, e, handoff("Apollo", "asgardeo", "asgardeo", testClock))

	if _, found, _ := store.Get(ctx, testIncidentID); found {
		t.Error("the SaaS SRE chain is still running after the handoff")
	}
	if got := notesWith(notes, "Escalated to SME"); len(got) != 1 {
		t.Errorf("ladder summaries naming the handoff = %d; want 1 (%v)", len(got), notes.notes)
	}
	if err := e.Tick(ctx, testClock.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(ladderChat.posted) != 0 {
		t.Errorf("%d ladder cards after the handoff; the chain must not climb", len(ladderChat.posted))
	}

	// Two SMEs on duty, neither ever called: email order picks s-a.
	if len(smeChat.posted) != 1 || smeChat.posted[0].RecipientName != "s-a" {
		t.Fatalf("SME cards = %+v; want exactly one, for s-a", smeChat.posted)
	}
	card := smeChat.posted[0]
	if card.RungRole != "Special Ops on duty" || card.Rule != "SME_HANDOFF" || !strings.Contains(card.Instruction, "Special Ops") {
		t.Errorf("card = %+v", card)
	}
	sme := notesWith(notes, "Special Ops (SME) Page")
	if len(sme) != 1 || !strings.Contains(sme[0], "[OK]") || !strings.Contains(sme[0], "s-a@example.com") ||
		!strings.Contains(sme[0], "SME_ASG_DAY") || !strings.Contains(sme[0], "asgardeo") {
		t.Errorf("SME work note = %v", sme)
	}
	if _, ok := store.called["s-a@example.com"]; !ok {
		t.Error("the paged SME was not recorded for the longest-since-called rule")
	}
}

// Of the SMEs on duty, whoever has gone longest without a call is paged.
func TestSME_PicksWhoeverWentLongestWithoutACall(t *testing.T) {
	smeChat, store := &fakeChat{}, newMemStore()
	store.called = map[string]time.Time{"s-a@example.com": testClock.Add(-time.Hour)}
	e := smeEngine(rotaReader(true), store, &fakeNotes{}, &fakeChat{}, smeChat, smeOn)

	raise(t, e, handoff("Apollo", "asgardeo", "asgardeo", testClock))
	if len(smeChat.posted) != 1 || smeChat.posted[0].RecipientName != "s-b" {
		t.Fatalf("SME cards = %+v; want one, for s-b, who was never called", smeChat.posted)
	}
}

// Only a SaaS SRE incident pages SME: IaaS SRE, a CRE team or no team at all
// is logged and ignored, and an IaaS chain keeps climbing.
func TestSME_GateRefusesAnythingButSaaSSRE(t *testing.T) {
	ctx := context.Background()
	for _, group := range []string{"SRE IaaS", "Atlas", "", "Somebody Else"} {
		t.Run(group, func(t *testing.T) {
			smeChat, store, notes := &fakeChat{}, newMemStore(), &fakeNotes{}
			e := smeEngine(rotaReader(true), store, notes, &fakeChat{}, smeChat, smeOn)
			if group == "SRE IaaS" {
				handle(t, e, created(t, group, "HIGH"))
			}
			raise(t, e, handoff(group, "asgardeo", "asgardeo", testClock))
			if len(smeChat.posted) != 0 || len(notes.notes) != 0 {
				t.Errorf("cards=%d notes=%v; want no page and no note", len(smeChat.posted), notes.notes)
			}
			if _, found, _ := store.Get(ctx, testIncidentID); group == "SRE IaaS" && !found {
				t.Error("the IaaS chain was stopped by a handoff that does not apply to it")
			}
		})
	}
}

// Without rota codes the gate falls back to the SRE team list and team.type:
// an SRE team typed sre-iaas is not SaaS.
func TestSME_GateWithoutRotaCodes(t *testing.T) {
	reader := rotaReader(false)
	reader.members = []teamMember{
		{TeamKey: "apollo", TeamType: "sre-abt", Email: "a-lead@example.com", Role: roleLead},
		{TeamKey: "iaas", TeamType: "sre-iaas", Email: "i-lead@example.com", Role: roleLead},
	}
	r := rotaResolver(reader, "")
	for group, want := range map[string]bool{"Apollo": true, "SRE IaaS": false, "Atlas": false, "": false} {
		got, err := r.SaaSSRETeam(context.Background(), group)
		if err != nil || got != want {
			t.Errorf("SaaSSRETeam(%q) = %v, %v; want %v", group, got, err, want)
		}
	}
}

// Pressed again for the same team while its page is open: ignored. A
// different team: its own call.
func TestSME_SameTeamIsDeduplicatedAnotherTeamPages(t *testing.T) {
	smeChat, store := &fakeChat{}, newMemStore()
	e := smeEngine(rotaReader(true), store, &fakeNotes{}, &fakeChat{}, smeChat, smeOn)

	raise(t, e, handoff("Apollo", "asgardeo", "asgardeo", testClock))
	raise(t, e, handoff("Apollo", "asgardeo", "asgardeo", testClock.Add(time.Minute)))
	if len(smeChat.posted) != 1 {
		t.Fatalf("%d SME cards after two presses for the same team; want 1", len(smeChat.posted))
	}
	// No smeTeam on the event: sme.teams maps the escalation team.
	raise(t, e, handoff("Apollo", "", "choreo", testClock.Add(2*time.Minute)))
	if len(smeChat.posted) != 2 || smeChat.posted[1].RecipientName != "c-1" {
		t.Fatalf("SME cards = %+v; want a second, for the Choreo SME", smeChat.posted)
	}
}

// An engineer assigned after the press closes the page: a later press pages
// again, while a replay of the press already answered does not.
func TestSME_AssignmentAfterTheHandoffClosesThePage(t *testing.T) {
	smeChat, store := &fakeChat{}, newMemStore()
	e := smeEngine(rotaReader(true), store, &fakeNotes{}, &fakeChat{}, smeChat, smeOn)

	first := handoff("Apollo", "asgardeo", "asgardeo", testClock)
	raise(t, e, first)
	handle(t, e, assigned(t))
	if len(store.smePages) != 0 {
		t.Fatalf("open SME pages after an assignment: %v", store.smePages)
	}
	raise(t, e, first)
	if len(smeChat.posted) != 1 {
		t.Fatalf("%d SME cards; a replay of an answered press must not page again", len(smeChat.posted))
	}
	raise(t, e, handoff("Apollo", "asgardeo", "asgardeo", testClock.Add(30*time.Minute)))
	if len(smeChat.posted) != 2 {
		t.Fatalf("%d SME cards; a press after the assignment should page again", len(smeChat.posted))
	}
}

// Nobody on duty for the SME team: no page, NO_RECIPIENTS on the incident,
// and nothing left open, so the next press tries again.
func TestSME_NobodyOnDuty(t *testing.T) {
	smeChat, store, notes := &fakeChat{}, newMemStore(), &fakeNotes{}
	e := smeEngine(rotaReader(true), store, notes, &fakeChat{}, smeChat, smeOn)

	raise(t, e, handoff("Apollo", "bijira", "bijira", testClock))
	if len(smeChat.posted) != 0 {
		t.Fatalf("%d SME cards with nobody on duty", len(smeChat.posted))
	}
	if got := notesWith(notes, "NO_RECIPIENTS"); len(got) != 1 || !strings.Contains(got[0], "bijira") {
		t.Errorf("work notes = %v; want NO_RECIPIENTS for bijira", notes.notes)
	}
	if len(store.smePages) != 0 {
		t.Errorf("a page that reached nobody was left open: %v", store.smePages)
	}
}

// An escalation team with no SME team, on the event or in sme.teams.
func TestSME_NoTeamMapped(t *testing.T) {
	smeChat, notes := &fakeChat{}, &fakeNotes{}
	e := smeEngine(rotaReader(true), newMemStore(), notes, &fakeChat{}, smeChat, smeOn)

	raise(t, e, handoff("Apollo", "", "devant", testClock))
	if len(smeChat.posted) != 0 || len(notesWith(notes, "NO_SME_TEAM")) != 1 {
		t.Errorf("cards=%d notes=%v; want no page and NO_SME_TEAM", len(smeChat.posted), notes.notes)
	}
}

// With the sme section absent the handoff is ignored entirely, as before the
// page existed: the chain keeps climbing.
func TestSME_DisabledIgnoresTheHandoff(t *testing.T) {
	ctx := context.Background()
	smeChat, store := &fakeChat{}, newMemStore()
	e := smeEngine(rotaReader(true), store, &fakeNotes{}, &fakeChat{}, smeChat, SMEConfig{})

	handle(t, e, created(t, "Apollo", "HIGH"))
	raise(t, e, handoff("Apollo", "asgardeo", "asgardeo", testClock))
	if _, found, _ := store.Get(ctx, testIncidentID); !found || len(smeChat.posted) != 0 {
		t.Errorf("ladder running=%v cards=%d; a disabled SME page changes nothing", found, len(smeChat.posted))
	}
}

// The paging engines' own consumers never act on the alert -- with
// INCIDENT_EVENT_HUB_TOPIC == sre-events the SRE engine reads that topic too,
// and only the dispatcher's path may page.
func TestSME_EngineConsumerIgnoresTheAlert(t *testing.T) {
	ctx := context.Background()
	smeChat, store := &fakeChat{}, newMemStore()
	e := smeEngine(rotaReader(true), store, &fakeNotes{}, &fakeChat{}, smeChat, smeOn)
	handle(t, e, created(t, "Apollo", "HIGH"))
	handle(t, e, alertRecord(t, handoff("Apollo", "asgardeo", "asgardeo", testClock)))
	if _, found, _ := store.Get(ctx, testIncidentID); !found || len(smeChat.posted) != 0 {
		t.Errorf("ladder running=%v cards=%d; the engine's consumer must not act on the alert", found, len(smeChat.posted))
	}
}

// The gate also matches the previous group by id, through sre.teams.aliases.
func TestSME_GateMatchesThePreviousGroupID(t *testing.T) {
	smeChat := &fakeChat{}
	e := smeEngine(rotaReader(true), newMemStore(), &fakeNotes{}, &fakeChat{}, smeChat, smeOn)
	teams := rotaTeams
	teams.Aliases = map[string]string{"SRE IaaS": "iaas", "group-apollo-uuid": "apollo"}
	e.resolver = NewTeamScheduleResolver(rotaReader(true), teams, nil)
	p := handoff("", "asgardeo", "asgardeo", testClock)
	p.PreviousAssignmentGroupID = "group-apollo-uuid"
	raise(t, e, p)
	if len(smeChat.posted) != 1 {
		t.Errorf("%d SME cards; the previous group's id should pass the gate", len(smeChat.posted))
	}
}

// The CRE engine never places the SME page.
func TestSME_CREEngineIgnoresTheAlert(t *testing.T) {
	smeChat := &fakeChat{}
	e := smeEngine(rotaReader(true), newMemStore(), &fakeNotes{}, &fakeChat{}, smeChat, smeOn)
	e.cfg.Kind = LadderCRE
	raise(t, e, handoff("Apollo", "asgardeo", "asgardeo", testClock))
	if len(smeChat.posted) != 0 {
		t.Errorf("the CRE engine paged SME")
	}
}

// On the call channel an SME with no dialable number is recorded, not paged,
// and safety.allowedNumbers is honoured.
func TestSME_CallChannelNeedsAnAllowedNumber(t *testing.T) {
	reader := rotaReader(true)
	caller, notes := &fakeCaller{}, &fakeNotes{}
	sme := SMEConfig{Enabled: true, Channel: ChannelCall, Safety: SMESafety{AllowedNumbers: []string{"+94770000001"}}}
	e := smeEngine(reader, newMemStore(), notes, &fakeChat{}, &fakeChat{}, sme)
	e.smeNotifiers = []notifier{voiceNotifier{calls: caller}}
	e.resolver = rotaResolver(reader, "").WithPhoneBook(PhoneBook{ByEmail: map[string]string{
		"s-a@example.com": "+94770000002", "c-1@example.com": "+94770000001",
	}})

	raise(t, e, handoff("Apollo", "asgardeo", "asgardeo", testClock))
	if len(caller.placed) != 0 || len(notesWith(notes, "NUMBER_NOT_ALLOWED")) != 1 {
		t.Fatalf("calls=%v notes=%v; want no call and NUMBER_NOT_ALLOWED", caller.placed, notes.notes)
	}
	raise(t, e, handoff("Apollo", "choreo-sme", "choreo", testClock))
	if len(caller.placed) != 1 || caller.placed[0].to != "+94770000001" {
		t.Fatalf("calls = %v; want one to the allowed number", caller.placed)
	}
}

// The sme section: off when absent, strict keys, a mapped team needs both
// sides.
func TestConfig_SMESection(t *testing.T) {
	cfg, err := loadYAML(t, "enabled: true\n")
	if err != nil || cfg.SME.Enabled {
		t.Fatalf("absent sme: %+v, %v; want disabled", cfg.SME, err)
	}
	cfg, err = loadYAML(t, `
sme:
  enabled: true
  channel: log
  chat: {webhookUrlEnv: SME_CHAT_WEBHOOK_URL, audience: Special Ops}
  teams: {Asgardeo: asgardeo}
  safety: {allowedNumbers: ["+94770000001"]}
`)
	if err != nil || !cfg.SME.Enabled || cfg.SME.Channel != ChannelLog || cfg.SME.TeamFor(" asgardeo ") != "asgardeo" {
		t.Fatalf("sme = %+v, %v", cfg.SME, err)
	}
	for name, body := range map[string]string{
		"unknown key":      "sme:\n  maxCallsPerLadder: 1\n",
		"unknown channel":  "sme:\n  channel: pager\n",
		"webhook URL":      "sme:\n  chat: {webhookUrlEnv: \"https://chat.example/x\"}\n",
		"empty team value": "sme:\n  teams: {asgardeo: \"\"}\n",
	} {
		if _, err := loadYAML(t, body); err == nil {
			t.Errorf("%s: loaded; want an error", name)
		}
	}
}
