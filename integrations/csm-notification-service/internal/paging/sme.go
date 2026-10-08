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
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

// The Special Ops (SME) page.
//
// SME -- Subject Matter Specialist -- is a separate team paged ONLY when an
// assigned SaaS SRE incident is escalated to a Special Ops team. entity-service
// publishes incident.special_ops_alert on the operations topic (sre-events)
// whenever an incident's assignment group changes INTO a Special Ops group --
// whatever made the change, which today is the "Escalate to Special Ops Team"
// button -- and only when it has SRE_EVENT_HUB_TOPIC set. The dispatcher's
// sre-events consumer hands it here (HandleSpecialOpsAlert); the paging
// engines' own consumers ignore the type, so one alert pages once. The SaaS
// SRE chain stops, and ONE call goes to the SME on duty for the current
// Day/Night window of the chosen SME team. Not a ladder: no rungs, no clock,
// nothing stored but the dedup key.
//
//	same SME team alerted again while its page is open   ignored
//	a different SME team                                 its own call
//	an engineer assigned afterwards                      closes every page,
//	                                                     so a later alert pages
//
// IaaS SRE incidents, CRE teams and unknown groups never page SME (the gate
// below). NOT BUILT: the rule that the button needs an assignee first (M2)
// belongs to the portal and is deferred.

// smePageTTL bounds an open SME page that no assignment ever closes.
const smePageTTL = 24 * time.Hour

// SpecialistResolver is implemented by a Resolver that can tell whether an
// assignment group is a SaaS SRE team -- the only kind an SME page may follow.
// A resolver that cannot (the roster) places no SME page at all.
type SpecialistResolver interface {
	SaaSSRETeam(ctx context.Context, group string) (bool, error)
}

// specialOpsPress is one move into a Special Ops group, as the page needs it.
type specialOpsPress struct {
	IncidentID, Number, Title, Product string
	// TeamKey/TeamLabel are the Special Ops team; SMETeam the rota team to
	// page when the publisher names it.
	TeamKey, TeamLabel, SMETeam string
	// PreviousGroup/PreviousGroupID are the group the incident left.
	PreviousGroup, PreviousGroupID string
	AssignmentGroup                string
	ChangedBy                      string
	At                             time.Time
}

// HandleSpecialOpsAlert places the SME page for incident.special_ops_alert;
// dispatch calls it from the sre-events consumer. An error is returned only
// for what a retry can fix (Redis, the rota unreachable, a transient call
// failure); everything else is logged and, where it concerns the incident,
// written onto it. Only the SRE engine pages.
func (e *Engine) HandleSpecialOpsAlert(ctx context.Context, incidentID string, p events.IncidentSpecialOpsAlertPayload) error {
	if e.cfg.Kind != LadderSRE {
		return nil
	}
	group := p.AssignmentGroupName
	if group == "" {
		group = p.AssignmentGroupID
	}
	return e.handleSpecialOps(ctx, specialOpsPress{
		IncidentID: incidentID, Number: p.Number, Title: p.Subject, Product: p.Product,
		TeamKey: p.TeamKey, TeamLabel: p.TeamLabel, SMETeam: p.SMETeam,
		PreviousGroup: p.PreviousAssignmentGroupName, PreviousGroupID: p.PreviousAssignmentGroupID,
		AssignmentGroup: group, ChangedBy: p.ChangedBy, At: reportedAt(p.ChangedOn),
	})
}

// isSaaSSREGroup applies the gate to the group the incident left, by name and
// then by id (either may be what sre.teams.aliases maps).
func (e *Engine) isSaaSSREGroup(ctx context.Context, gate SpecialistResolver, pr specialOpsPress) (bool, error) {
	for _, g := range []string{pr.PreviousGroup, pr.PreviousGroupID} {
		if strings.TrimSpace(g) == "" {
			continue
		}
		ok, err := gate.SaaSSRETeam(ctx, g)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

// handleSpecialOps places the SME page for one alert; see the top of this file.
func (e *Engine) handleSpecialOps(ctx context.Context, pr specialOpsPress) error {
	incidentID := pr.IncidentID
	log := slog.With("incidentId", incidentID, "incident", pr.Number, "specialOpsTeam", pr.TeamKey)
	if !e.cfg.SME.Enabled {
		log.InfoContext(ctx, "escalation: SME paging is off (sme.enabled); ignoring the Special Ops alert")
		return nil
	}
	if strings.TrimSpace(incidentID) == "" {
		log.WarnContext(ctx, "escalation: Special Ops alert carries no incident id; no SME page")
		return nil
	}
	gate, ok := e.resolver.(SpecialistResolver)
	if !ok {
		log.WarnContext(ctx, "escalation: this resolver cannot read an SME rota; no SME page")
		return nil
	}
	saas, err := e.isSaaSSREGroup(ctx, gate, pr)
	if err != nil {
		return fmt.Errorf("escalation: tell whether %s was a SaaS SRE incident: %w", incidentID, err)
	}
	if !saas {
		log.InfoContext(ctx, "escalation: the incident did not come from a SaaS SRE team; no SME page",
			"previousAssignmentGroup", pr.PreviousGroup, "previousAssignmentGroupId", pr.PreviousGroupID)
		return nil
	}

	// The SaaS SRE chain stops whatever happens to the page: the incident is
	// Special Ops' now.
	if err := e.stopForHandoff(ctx, incidentID); err != nil {
		return err
	}

	team := teamKeyFor(pr.SMETeam)
	if team == "" {
		team = e.cfg.SME.TeamFor(pr.TeamKey)
	}
	t := smeTrigger(pr, team)
	if team == "" {
		log.WarnContext(ctx, "escalation: no SME team mapped for this Special Ops team; no SME page")
		e.writeSMENote(ctx, t, pr, nil, "NO_SME_TEAM",
			fmt.Sprintf("no SME team mapped for Special Ops team %s", orNone(pr.TeamKey)))
		return nil
	}
	log = log.With("smeTeam", team)

	// A replay of an alert an assignment has already answered is not a new
	// one, even though its page key is gone.
	closed, err := e.store.SMEClosedThrough(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("escalation: read closed SME pages for %s: %w", incidentID, err)
	}
	if !closed.IsZero() && !pr.At.After(closed) {
		log.InfoContext(ctx, "escalation: ignored, SME page already answered (a replay of an earlier alert)")
		return nil
	}

	opened, err := e.store.OpenSMEPage(ctx, incidentID, team, pr.At, smePageTTL)
	if err != nil {
		e.releaseSMEPage(ctx, incidentID, team)
		return fmt.Errorf("escalation: open SME page for %s: %w", incidentID, err)
	}
	if !opened {
		log.InfoContext(ctx, "escalation: ignored, SME page already open")
		return nil
	}

	paged, err := e.pageSME(ctx, t, pr)
	if err != nil || !paged {
		// Nobody was reached, so nothing is open: a retry, or the next
		// alert, tries again.
		e.releaseSMEPage(ctx, incidentID, team)
	}
	return err
}

// stopForHandoff stops the incident's running SRE chain, writing its summary
// the way any other cancellation does.
func (e *Engine) stopForHandoff(ctx context.Context, incidentID string) error {
	st, found, err := e.store.Get(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("escalation: load ladder for %s: %w", incidentID, err)
	}
	if !found {
		return nil
	}
	return e.stopLadder(ctx, incidentID, st, cancelEscalatedToSME)
}

// smeTrigger is the one-call plan's trigger for an alert.
func smeTrigger(pr specialOpsPress, team string) Trigger {
	label := pr.TeamLabel
	if label == "" {
		label = pr.TeamKey
	}
	return Trigger{
		IncidentID: pr.IncidentID,
		Number:     pr.Number,
		Title:      pr.Title,
		Team:       label,
		Kind:       TriggerSpecialOps,
		At:         pr.At,
		Routing: RoutingContext{
			Product:         pr.Product,
			AssignedCRETeam: pr.AssignmentGroup,
			Ladder:          LadderSME,
			SMETeam:         team,
			At:              pr.At,
		},
	}
}

// pageSME finds the ONE SME on duty and pages them. It reports whether
// somebody was reached; an error means a retry may do better.
func (e *Engine) pageSME(ctx context.Context, t Trigger, p specialOpsPress) (bool, error) {
	sme := e.cfg.SME
	people, err := e.resolver.Resolve(ctx, Level0, t.Routing)
	if err != nil {
		return false, fmt.Errorf("escalation: resolve the SME on duty for %s: %w", t.IncidentID, err)
	}
	if len(people) == 0 {
		slog.WarnContext(ctx, "escalation: nobody on duty for the SME team; no SME page",
			"incidentId", t.IncidentID, "smeTeam", t.Routing.SMETeam, "at", t.At.Format(time.RFC3339))
		e.writeSMENote(ctx, t, p, nil, "NO_RECIPIENTS",
			fmt.Sprintf("nobody on duty for SME team %s at %s", t.Routing.SMETeam, istStamp(t.At)))
		return false, nil
	}
	who := people[0]

	if sme.Channel.Uses(ChannelCall) {
		reason := "NO_NUMBER"
		if who.Phone != "" && !sme.Dialable(who.Phone) {
			who.Phone, reason = "", "NUMBER_NOT_ALLOWED"
		}
		if who.Phone == "" && sme.Channel == ChannelCall {
			slog.WarnContext(ctx, "escalation: the SME on duty cannot be called; no SME page",
				"incidentId", t.IncidentID, "smeTeam", t.Routing.SMETeam, "recipient", who.Name, "reason", reason)
			e.writeSMENote(ctx, t, p, &who, reason, "")
			return false, nil
		}
	}

	if len(e.smeNotifiers) == 0 && e.cfg.CallSendingEnabled {
		// sme.channel names a channel with no client (logged at startup).
		// Unlike a ladder rung this is the only page, so it is not recorded
		// as sent.
		slog.ErrorContext(ctx, "escalation: no notifier for the SME page's channel; nobody was contacted",
			"incidentId", t.IncidentID, "channel", string(sme.Channel))
		e.writeSMENote(ctx, t, p, &who, "NO_CHANNEL", string(sme.Channel))
		return false, nil
	}
	plan := Plan{Trigger: t, Calls: []PlannedCall{{Level: Level0, Ordinal: 1, At: t.At, Recipient: who}}}
	if err := e.placeVia(ctx, plan, plan.Calls[0], e.smeNotifiers, sme.Channel); err != nil {
		if !isPermanent(err) {
			return false, fmt.Errorf("escalation: page the SME for %s: %w", t.IncidentID, err)
		}
		slog.ErrorContext(ctx, "escalation: SME page rejected by the provider; not retrying",
			"incidentId", t.IncidentID, "smeTeam", t.Routing.SMETeam, "reason", permanentReason(err))
		e.writeSMENote(ctx, t, p, &who, "CALL_FAILED", permanentReason(err))
		return false, nil
	}
	slog.InfoContext(ctx, "escalation: SME paged",
		"incidentId", t.IncidentID, "smeTeam", t.Routing.SMETeam, "recipient", who.Name,
		"shift", who.ShiftCode, "channel", string(sme.Channel))
	e.writeSMENote(ctx, t, p, &who, "", "")
	return true, nil
}

// releaseSMEPage drops a page key that reached nobody. Best-effort: a key left
// behind only ignores the next press for the same team until it expires.
func (e *Engine) releaseSMEPage(ctx context.Context, incidentID, team string) {
	if err := e.store.CloseSMEPage(ctx, incidentID, team); err != nil {
		slog.WarnContext(ctx, "escalation: could not release an SME page that reached nobody; "+
			"a press for this team is ignored until it expires",
			"incidentId", incidentID, "smeTeam", team, "err", err)
	}
}

// closeSMEPages closes every SME page open on the incident, on an assignment.
func (e *Engine) closeSMEPages(ctx context.Context, incidentID string) error {
	if e.cfg.Kind != LadderSRE || !e.cfg.SME.Enabled {
		return nil
	}
	n, err := e.store.CloseSMEPages(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("escalation: close SME pages for %s: %w", incidentID, err)
	}
	if n > 0 {
		slog.InfoContext(ctx, "escalation: engineer assigned; SME pages closed", "incidentId", incidentID, "closed", n)
	}
	return nil
}

// writeSMENote records the press on the incident: who was paged for which
// SME team and window, or why nobody was. who is nil when nobody was found;
// failure is empty when the page went out. Never fails the record, as
// writeNote.
func (e *Engine) writeSMENote(ctx context.Context, t Trigger, p specialOpsPress, who *Recipient, failure, detail string) {
	const stamp = "2006-01-02 15:04:05"
	var b strings.Builder
	b.WriteString("Execution Summary Of the Special Ops (SME) Page\n\n")
	b.WriteString(fmt.Sprintf("Escalated to Special Ops: %s by %s\n", t.At.Format(stamp), orNone(p.ChangedBy)))
	b.WriteString(fmt.Sprintf("Special Ops team: %s; SME team: %s; previous assignment group: %s\n\n",
		orNone(t.Team), orNone(t.Routing.SMETeam), orNone(p.PreviousGroup)))
	switch {
	case who == nil:
		b.WriteString(fmt.Sprintf("[%s][SME][ERROR][%s][%s]", t.At.Format(stamp), failure, detail))
	case failure != "":
		b.WriteString(fmt.Sprintf("[%s][SME][ERROR][%s][%s][shift %s]", t.At.Format(stamp), failure, who.Email, orNone(who.ShiftCode)))
		if detail != "" {
			b.WriteString(fmt.Sprintf("[%s]", detail))
		}
	default:
		b.WriteString(fmt.Sprintf("[%s][SME][OK][Page via %s][%s][shift %s]",
			t.At.Format(stamp), e.cfg.SME.Channel, who.Email, orNone(who.ShiftCode)))
	}
	note := b.String()
	if e.notes == nil {
		slog.InfoContext(ctx, "escalation: no incident-notes client configured; SME page summary not written back",
			"incidentId", t.IncidentID)
		return
	}
	if err := e.notes.AppendWorkNote(ctx, t.IncidentID, note); err != nil {
		slog.ErrorContext(ctx, "escalation: could not write the SME page summary; it is lost for this incident",
			"incidentId", t.IncidentID, "error", err)
	}
}
