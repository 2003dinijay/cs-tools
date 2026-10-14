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

import { Suspense, useRef, useState, type ReactNode } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Button, Chip, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { useQueryClient, useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import type { IncidentDetail, IncidentPatchPayloadDto, IncidentState } from "@src/types";
import { currentUser } from "@src/services/currentUser";
import { incidents } from "@src/services/incidents";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { ErrorState } from "@components/support/ErrorState";
import { SectionCard } from "@components/case-detail/SectionCard";
import { CaseActivityFeed } from "@components/case-detail/CaseActivityFeed";
import { ACTION_BAR_CONTROL_HEIGHT, StateSelect } from "@components/case-detail/CaseActionBar";
import { IncidentResolutionDialog } from "@components/operations/IncidentResolutionDialog";
import { toApiError } from "@utils/ApiError";
import {
  incidentPriorityColor,
  incidentPriorityLabel,
  incidentStateColor,
  incidentStateLabel,
  INCIDENT_STATE_TRANSITIONS,
} from "@components/operations/incidentConfig";
import { formatDate } from "@utils/dateTime";

// Read-only counterpart of the webapp's CsmIncidentDetailPage.tsx (no Edit dialog, no comment
// composer, no attachments — see the scope note on OperationsPage.tsx). Layout follows this app's
// own ChangeRequestDetailPage.tsx: a stack of SectionCards with DetailRow/TextBlock fields rather
// than the webapp's responsive MetaCell grid.
export default function IncidentDetailPage() {
  const { id } = useParams<{ id: string }>();

  return (
    <Stack gap={2} sx={{ mb: 10 }}>
      <IncidentDetailErrorBoundary>
        <Suspense fallback={<IncidentDetailSkeleton />}>
          <IncidentDetailContent id={id ?? ""} />
        </Suspense>
      </IncidentDetailErrorBoundary>
    </Stack>
  );
}

function IncidentDetailContent({ id }: { id: string }) {
  const { data: incident } = useSuspenseQuery(incidents.get(id));

  const hasLinks = !!(incident.changeRequest || incident.problem || incident.causedBy);
  const hasNotes = !!(incident.additionalComments || incident.workNotes);

  return (
    <>
      <Stack gap={1}>
        <Typography variant="subtitle2" color="text.secondary" noWrap>
          {incident.number}
        </Typography>

        <Typography variant="h6">{incident.subject}</Typography>

        <Stack direction="row" gap={1} flexWrap="wrap">
          {incident.state && (
            <Chip size="small" label={incidentStateLabel(incident.state)} color={incidentStateColor(incident.state)} />
          )}
          {incident.priority && (
            <Chip
              size="small"
              variant="outlined"
              label={incidentPriorityLabel(incident.priority)}
              color={incidentPriorityColor(incident.priority)}
            />
          )}
        </Stack>

        <IncidentActionBar incidentId={id} incident={incident} />
      </Stack>

      <SectionCard title="Overview">
        <Stack gap={1}>
          <DetailRow label="Caller" value={incident.caller?.name} />
          <DetailRow label="Category" value={incident.category} />
          <DetailRow label="Subcategory" value={incident.subcategory} />
          <DetailRow label="Contact Type" value={incident.contactType} />
          <DetailRow label="Impact" value={incident.impact} />
          <DetailRow label="Urgency" value={incident.urgency} />
          <DetailRow label="Service" value={incident.service?.name} />
          <DetailRow label="Service Offering" value={incident.serviceOffering?.name} />
          <DetailRow label="Configuration Item" value={incident.configurationItem?.name} />
          <DetailRow label="Assignment Group" value={incident.assignmentGroup?.name} />
          <DetailRow label="Assigned To" value={incident.assignedTo?.name} />
          <DetailRow label="Opened" value={incident.openedOn ? formatDate(incident.openedOn) : undefined} />
          <DetailRow label="Created By" value={incident.createdBy} />
          <DetailRow label="Created On" value={incident.createdOn ? formatDate(incident.createdOn) : undefined} />
          <DetailRow label="Updated On" value={incident.updatedOn ? formatDate(incident.updatedOn) : undefined} />
        </Stack>
      </SectionCard>

      {hasLinks && (
        <SectionCard title="Linked records">
          <Stack gap={1}>
            <DetailRow label="Change Request" value={incident.changeRequest?.name} />
            <DetailRow label="Problem" value={incident.problem?.name} />
            {/* "Caused by" has no confirmed target record type (could be a change request, a
                problem, or something else), same caveat the webapp's own detail page notes — left
                as plain text rather than guessing a route. */}
            <DetailRow label="Caused By" value={incident.causedBy?.name} />
          </Stack>
        </SectionCard>
      )}

      {hasNotes && (
        <SectionCard title="Comments & notes">
          <Stack gap={1.5}>
            <TextBlock label="Additional comments (customer-visible)" value={incident.additionalComments} />
            <TextBlock label="Internal work notes" value={incident.workNotes} />
          </Stack>
        </SectionCard>
      )}

      {incident.watchList.length > 0 && (
        <SectionCard title="Watch list">
          <Stack direction="row" gap={1} flexWrap="wrap">
            {incident.watchList.map((w) => (
              <Chip key={w.id} size="small" variant="outlined" label={w.name || w.email} />
            ))}
          </Stack>
        </SectionCard>
      )}

      {incident.linkedServiceRequests.length > 0 && <LinkedServiceRequests items={incident.linkedServiceRequests} />}

      <SectionCard title="Comments">
        <IncidentCommentsSection incidentId={id} />
      </SectionCard>
    </>
  );
}

// States where the work is over — nothing left to pick up, so no "Assign to me".
const DONE_INCIDENT_STATES: ReadonlySet<IncidentState> = new Set(["RESOLVED", "CLOSED", "CANCELLED"]);

type ResolutionTarget = "RESOLVED" | "CLOSED";

// Lifecycle actions for the incident, following the case detail page: "Assign to me" while the
// incident isn't the caller's, then a "Change state" dropdown of the legal next states once it is.
//
// "Assign to me" claims the incident and, when it hasn't been started yet, moves it to In Progress
// in the same PATCH — the same claim the webapp's CsmIncidentDetailPage makes when an unassigned
// incident is moved to IN_PROGRESS. An already-started incident only changes assignee.
// Resolved/Closed go through IncidentResolutionDialog first, since ServiceNow rejects them without
// a resolution code and notes.
function IncidentActionBar({ incidentId, incident }: { incidentId: string; incident: IncidentDetail }) {
  const queryClient = useQueryClient();
  const { data: currentUserId } = useSuspenseQuery(currentUser.id());
  const [isPending, setIsPending] = useState(false);
  // Synchronous in-flight guard: `isPending` only disables the controls after a re-render, so two
  // fast taps in the same tick could otherwise both send a PATCH.
  const inFlight = useRef(false);
  const [error, setError] = useState<string | null>(null);
  const [resolutionTarget, setResolutionTarget] = useState<ResolutionTarget | null>(null);

  if (!currentUserId || !incident.state) return null;
  const state = incident.state;
  const isMine = incident.assignedTo?.id === currentUserId;
  const targets = INCIDENT_STATE_TRANSITIONS[state] ?? [];

  const runPatch = (payload: IncidentPatchPayloadDto, fallbackMessage: string): Promise<void> => {
    if (inFlight.current) return Promise.resolve();
    inFlight.current = true;
    setIsPending(true);
    setError(null);
    return incidents
      .patch(incidentId, payload)
      .then(() => {
        setResolutionTarget(null);
        void queryClient.invalidateQueries({ queryKey: ["incident", incidentId] });
        void queryClient.invalidateQueries({ queryKey: ["incidents"] });
      })
      .catch((err) => setError(toApiError(err, fallbackMessage).message))
      .finally(() => {
        inFlight.current = false;
        setIsPending(false);
      });
  };

  const handleAssign = (): void => {
    void runPatch(
      { assignedEngineerId: currentUserId, ...(state === "NEW" ? { state: "IN_PROGRESS" as const } : {}) },
      "Could not assign the incident to you.",
    );
  };

  const handleTarget = (target: IncidentState): void => {
    if (target === "RESOLVED" || target === "CLOSED") {
      setResolutionTarget(target);
      return;
    }
    void runPatch({ state: target }, "Could not update the incident's state.");
  };

  let action: ReactNode = null;
  if (!isMine && !DONE_INCIDENT_STATES.has(state)) {
    action = (
      <Button
        variant="contained"
        size="small"
        disabled={isPending}
        onClick={handleAssign}
        sx={{ borderRadius: 999, height: ACTION_BAR_CONTROL_HEIGHT }}
      >
        Assign to me
      </Button>
    );
  } else if (isMine && targets.length > 0) {
    action = (
      <StateSelect targets={targets} labelFor={incidentStateLabel} disabled={isPending} onSelect={handleTarget} />
    );
  }

  if (!action) return null;

  return (
    <Stack gap={0.5} alignItems="flex-start">
      {action}
      {error && (
        <Typography variant="caption" color="error.main">
          {error}
        </Typography>
      )}
      {resolutionTarget && (
        <IncidentResolutionDialog
          target={resolutionTarget}
          isSubmitting={isPending}
          onClose={() => setResolutionTarget(null)}
          onSubmit={(fields) =>
            void runPatch({ state: resolutionTarget, ...fields }, "Could not update the incident's state.")
          }
        />
      )}
    </Stack>
  );
}

function LinkedServiceRequests({ items }: { items: { id: string; number: string; name: string }[] }) {
  const navigate = useNavigate();

  return (
    <SectionCard title="Linked service requests">
      <Stack direction="row" gap={1} flexWrap="wrap">
        {items.map((sr) => (
          <Chip
            key={sr.id}
            size="small"
            variant="outlined"
            clickable
            label={`${sr.number} — ${sr.name}`}
            onClick={() => navigate(`/cases/${sr.id}`)}
            sx={{ fontWeight: 600 }}
          />
        ))}
      </Stack>
    </SectionCard>
  );
}

// Own Suspense/error boundary, mirroring CaseActivityFeed.tsx's own CaseActivitiesTab: comments
// load independently of the rest of the detail page instead of blocking it.
function IncidentCommentsSection({ incidentId }: { incidentId: string }) {
  return (
    <IncidentCommentsErrorBoundary>
      <Suspense fallback={<Skeleton variant="rounded" height={56} />}>
        <IncidentCommentsContent incidentId={incidentId} />
      </Suspense>
    </IncidentCommentsErrorBoundary>
  );
}

function IncidentCommentsContent({ incidentId }: { incidentId: string }) {
  const { data: comments } = useSuspenseQuery(incidents.comments(incidentId));
  return <CaseActivityFeed comments={comments} audit={[]} attachments={[]} />;
}

function IncidentCommentsErrorBoundary({ children }: { children: ReactNode }) {
  const { reset } = useQueryErrorResetBoundary();

  return (
    <ErrorBoundary
      fallback={(_error, resetErrorBoundary) => (
        <ErrorState
          onRetry={() => {
            reset();
            resetErrorBoundary();
          }}
        />
      )}
    >
      {children}
    </ErrorBoundary>
  );
}

function DetailRow({ label, value }: { label: string; value?: string | null }) {
  if (!value) return null;

  return (
    <Stack direction="row" justifyContent="space-between" gap={2}>
      <Typography variant="body2" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2" color="text.primary" textAlign="right">
        {value}
      </Typography>
    </Stack>
  );
}

function TextBlock({ label, value }: { label: string; value?: string | null }) {
  if (!value || value.trim().length === 0) return null;

  return (
    <Stack gap={0.25}>
      <Typography variant="subtitle2" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2" color="text.primary" sx={{ whiteSpace: "pre-wrap" }}>
        {value}
      </Typography>
    </Stack>
  );
}

function IncidentDetailSkeleton() {
  return (
    <Stack gap={2}>
      <Skeleton variant="text" width="40%" height={24} />
      <Skeleton variant="text" width="80%" height={32} />
      <Skeleton variant="rounded" height={200} />
      <Skeleton variant="rounded" height={120} />
    </Stack>
  );
}

function IncidentDetailErrorBoundary({ children }: { children: ReactNode }) {
  const { reset } = useQueryErrorResetBoundary();

  return (
    <ErrorBoundary
      fallback={(_error, resetErrorBoundary) => (
        <ErrorState
          onRetry={() => {
            reset();
            resetErrorBoundary();
          }}
        />
      )}
    >
      {children}
    </ErrorBoundary>
  );
}
