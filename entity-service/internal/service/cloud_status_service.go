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
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// cloudStatusMaxAttempts caps how many times one webhook is retried before it
// stops being handed out as pending.
//
// ServiceNow set its REST step's retry policy to NONE: one attempt, and a
// failed post was lost silently. Retrying is a deliberate improvement, but a
// bounded one -- an endpoint that is refusing everything should not have the
// whole backlog thrown at it once a minute forever. Exhausted rows stay in the
// table, undelivered, which is what makes the failure visible instead of
// merely absent.
const cloudStatusMaxAttempts = 5

// cloudStatusPendingLimit caps one delivery batch.
const cloudStatusPendingLimit = 50

type cloudStatusService struct {
	repo repository.CloudStatusRepository
	// parentServiceIDs is the trigger's 14 service ids, from configuration.
	//
	// NOT hardcoded, despite being stable: ServiceNow kept them in the flow
	// definition where changing the list was a flow edit. Config is the
	// nearest equivalent that does not require a deploy, and it is the list
	// most likely to change -- adding a cloud to the status page is a
	// business decision, not a code change.
	parentServiceIDs []string
}

// NewCloudStatusService constructs the cloud status webhook decision service.
func NewCloudStatusService(repo repository.CloudStatusRepository, parentServiceIDs []string) CloudStatusService {
	return &cloudStatusService{repo: repo, parentServiceIDs: parentServiceIDs}
}

// Sweep decides which in-scope outages now owe the status dashboard a webhook
// and records them. It sends nothing itself.
//
// WHY A SWEEP AND NOT AN EVENT. ServiceNow triggered on the outage record
// being updated. In Postgres the `outage` table is sync output -- csm-sync-service
// writes it and nothing in this service does -- so there is no write here to
// hang a trigger or an outbox row off. A sweep over the current state is the
// only honest way to notice a transition, and it has a property the trigger
// lacked: it is self-healing. A sweep missed for any reason is made up by the
// next one, because the decision is derived from the outage's state rather
// than from having observed the change.
func (s *cloudStatusService) Sweep(ctx context.Context) (domain.CloudStatusSweepResponse, error) {
	if len(s.parentServiceIDs) == 0 {
		// Not an error that should fail the sweep loudly on every tick, but
		// also not something to pass over in silence: with no scope the sweep
		// is a no-op, and an operator who set the config wrongly would
		// otherwise see only a healthy-looking zero.
		slog.WarnContext(ctx, "cloud status sweep has no in-scope services configured; nothing will ever be sent")
		return domain.CloudStatusSweepResponse{}, nil
	}

	candidates, err := s.repo.Candidates(ctx, s.parentServiceIDs)
	if err != nil {
		return domain.CloudStatusSweepResponse{}, err
	}

	resp := domain.CloudStatusSweepResponse{Scanned: len(candidates)}
	for _, c := range candidates {
		if domain.CloudOfferingSlug(c.Cloud) == "" {
			// No cloud monitor on the outage's configuration item, or an
			// offering this service does not know how to address. Either way
			// there is nowhere to post.
			//
			// ServiceNow's Look Up Record step failed the whole flow execution
			// here, which meant one unroutable outage stopped every later step
			// for that record. Skipping and continuing is the right trade for
			// a sweep that handles many outages per run -- but it must be
			// counted, or an outage that never reaches the dashboard looks
			// exactly like an outage with nothing to report.
			resp.SkippedNoCloud++
			slog.WarnContext(ctx, "in-scope outage has no routable cloud offering; skipping",
				"outageId", c.OutageID, "number", c.Number, "cloudOffering", c.Cloud)
			continue
		}
		recorded, err := s.repo.Record(ctx, c)
		if err != nil {
			return domain.CloudStatusSweepResponse{}, err
		}
		if recorded {
			resp.Recorded++
			slog.InfoContext(ctx, "cloud status transition recorded",
				"outageId", c.OutageID, "number", c.Number,
				"event", string(c.Event), "cloud", c.Cloud)
		}

		// The status write runs on EVERY sweep, not only when the transition
		// was newly recorded.
		//
		// Recording is once-only because a webhook must not be re-sent; the
		// status is the opposite kind of thing. It is a desired end state, and
		// re-asserting it is how the port self-heals -- if the sync overwrites
		// a monitor, or a row was missed, the next sweep puts it right. The
		// repository skips monitors already showing the target value, so the
		// steady state costs a read and no writes.
		changed, unknownType, err := s.applyMonitorStatus(ctx, c)
		if err != nil {
			return domain.CloudStatusSweepResponse{}, err
		}
		resp.MonitorsUpdated += changed
		if unknownType {
			resp.UnknownOutageType++
		}
	}
	return resp, nil
}

// applyMonitorStatus writes the status every monitor affected by this outage
// should currently show. It reports how many rows changed and whether the
// outage's type had to be guessed at.
//
// This is steps 3-6 and 10-13 of the flow. The two arms differ only in the
// status they write: a completed outage returns its monitors to OPERATIONAL,
// an ongoing one sets the severity its type implies.
func (s *cloudStatusService) applyMonitorStatus(ctx context.Context, c repository.CloudStatusCandidate) (int64, bool, error) {
	var status domain.CloudMonitorStatus
	var unknownType bool

	if c.Event == domain.CloudStatusEventOutageEnd {
		// The completed arm wrote a literal 0. Note what it did NOT do: it did
		// not check whether some OTHER ongoing outage also affects these
		// monitors. Two overlapping outages on one component mean the first to
		// end clears the second's status, and the page shows Operational while
		// an incident is still running.
		//
		// That is faithfully reproduced here rather than fixed, because fixing
		// it changes what the public page says and needs a decision from
		// whoever owns it -- see this port's own notes. It is recorded so the
		// next person does not have to rediscover it from behaviour.
		status = domain.CloudMonitorStatusOperational
	} else {
		mapped, ok := domain.StatusForOngoingOutage(c.Type)
		if !ok {
			status = domain.CloudMonitorStatusUnknownType
			unknownType = true
			slog.ErrorContext(ctx, "ongoing outage has no usable type; falling back rather than leaving the status page claiming all is well",
				"outageId", c.OutageID, "number", c.Number,
				"outageType", c.Type, "fallback", string(status))
		} else {
			status = mapped
		}
	}

	monitors, err := s.repo.AffectedMonitors(ctx, c.OutageID)
	if err != nil {
		return 0, unknownType, err
	}
	if len(monitors) == 0 {
		// Common and not an error: most outages name no affected CIs at all,
		// and a CI that is not a service offering has no monitor. The webhook
		// still goes out -- it is routed from the outage's own configuration
		// item, not from this list.
		return 0, unknownType, nil
	}

	changed, err := s.repo.SetMonitorStatus(ctx, monitors, status)
	if err != nil {
		return 0, unknownType, err
	}
	if changed > 0 {
		slog.InfoContext(ctx, "cloud monitor status updated",
			"outageId", c.OutageID, "number", c.Number,
			"status", string(status), "monitorsChanged", changed)
	}
	return changed, unknownType, nil
}

// PendingWebhooks returns the webhooks still owed to the dashboard, for the
// delivering task to post.
//
// The cloud is translated to its wire slug here rather than in SQL so that the
// mapping lives in one place next to the reason it exists.
func (s *cloudStatusService) PendingWebhooks(ctx context.Context) (domain.PendingCloudStatusWebhooksResponse, error) {
	rows, err := s.repo.Pending(ctx, cloudStatusPendingLimit, cloudStatusMaxAttempts)
	if err != nil {
		return domain.PendingCloudStatusWebhooksResponse{}, err
	}

	out := make([]domain.PendingCloudStatusWebhook, 0, len(rows))
	for _, w := range rows {
		slug := domain.CloudOfferingSlug(w.Cloud)
		if slug == "" {
			// Recorded under an offering this build cannot address. Only
			// reachable if the sync adds an enum value ahead of this service
			// knowing about it, so it is worth a loud line rather than a
			// silent drop.
			slog.ErrorContext(ctx, "pending cloud status webhook has an unmappable offering; not dispatching",
				"webhookId", w.ID, "cloudOffering", w.Cloud)
			continue
		}
		w.Cloud = slug
		out = append(out, w)
	}
	return domain.PendingCloudStatusWebhooksResponse{Count: len(out), Webhooks: out}, nil
}

// RecordDelivery stamps the outcome of one webhook attempt.
func (s *cloudStatusService) RecordDelivery(ctx context.Context, req domain.RecordCloudStatusDeliveryRequest) error {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return err
	}
	if !req.Delivered && strings.TrimSpace(req.Error) == "" {
		return &apierror.ValidationError{Msg: "error is required when delivered is false"}
	}
	return s.repo.RecordDelivery(ctx, req.ID, req.Delivered, req.Error)
}
