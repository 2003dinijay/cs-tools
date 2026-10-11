import { TIMELINE_META } from "@root/src/config/constants";
import { ProgressTimelineEntrySkeleton, TimelineEntry } from "./TimelineEntry";
import { Timeline } from "../../ui";
import type { CasesFiltersDto, ChangeRequest } from "@root/src/types";
import { useQuery } from "@tanstack/react-query";
import { changeRequests } from "@root/src/services/changes";
import { useFilters } from "@root/src/context/filters";
import {
  approvalEvidence,
  NO_EVIDENCE,
  timelineStepStates,
  type TimelineEvidence,
} from "@root/src/utils/changeRequestProgress";

interface ProgressTimelineViewProps {
  /** What the record says of the steps that need one; nothing when the approvals could not be read. */
  evidence?: TimelineEvidence;
  activeIndex: number;
}

function ProgressTimelineView({ evidence = NO_EVIDENCE, activeIndex }: ProgressTimelineViewProps) {
  const steps = timelineStepStates(
    TIMELINE_META.map((step) => step.title),
    activeIndex,
    evidence,
  );

  return (
    <Timeline>
      {TIMELINE_META.map((step, index) => (
        <TimelineEntry
          key={index}
          variant="progress"
          status={steps[index].status}
          title={step.title}
          description={steps[index].description ?? step.description}
          fill={steps[index].fill}
          passed={steps[index].passed}
          end={steps[index].end}
          last={steps[index].last}
        />
      ))}
    </Timeline>
  );
}

function ProgressTimelineSkeletonView() {
  return (
    <Timeline>
      {TIMELINE_META.map((_, index) => (
        <ProgressTimelineEntrySkeleton key={index} last={index === TIMELINE_META.length - 1} />
      ))}
    </Timeline>
  );
}

export function useProgressTimelineModel(data?: ChangeRequest, filters?: CasesFiltersDto) {
  const status = filters?.changeRequestStates.find((t) => t.id === data?.statusId)?.label;

  const activeIndex = status ? TIMELINE_META.findIndex((e) => e.title === status) : -1;

  return {
    activeIndex,
  };
}

export function ProgressTimeline({ id }: { id: string }) {
  const { data, isLoading } = useQuery(changeRequests.get(id));
  // The approval stages are the timeline's evidence: a step before the current one is drawn done only when they
  // (or the detail) say it happened. A read that fails leaves the timeline with no evidence, which it draws as
  // such (a dashed ring, "No approval recorded"), never as a tick.
  const {
    data: approvals,
    isLoading: isApprovalsLoading,
    isError: isApprovalsError,
  } = useQuery(changeRequests.approvals(id));
  const { data: filters, isLoading: isFiltersLoading } = useFilters();
  const { activeIndex } = useProgressTimelineModel(data, filters);

  if (isLoading || isFiltersLoading || (isApprovalsLoading && !isApprovalsError) || !data || !filters)
    return <ProgressTimelineSkeletonView />;

  // A time WSO2 accepted (and the change moved on because of it) counts as the customer's approval here: the flag
  // itself stays false. The Customer Approval step is green for either, never a colour that says "not approved"
  // (see `customerApprovalStepFill`).
  return (
    <ProgressTimelineView evidence={approvalEvidence(data, approvals?.approvals ?? null)} activeIndex={activeIndex} />
  );
}
