/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { RotaReminderRequest, ScheduleAssignment, ScheduleAssignmentsResponse } from "../types";
// vi.mock below is hoisted above this import, so the section gets the mocks.
import RotaReminderSendNow from "./RotaReminderSendNow";

const requests: { data: RotaReminderRequest[] | undefined } = { data: [] };
const roster: { data: ScheduleAssignmentsResponse | undefined; isLoading: boolean; isError: boolean } = {
  data: undefined,
  isLoading: false,
  isError: false,
};
const sending = { pending: false };
const mutate = vi.fn();
const previewArgs = vi.fn();

vi.mock("../api/useRotaReminderRequests", () => ({
  useRotaReminderRequests: () => ({ data: requests.data }),
  useSendRotaReminder: () => ({ mutate, isPending: sending.pending }),
  useReminderPreview: (from: string, to: string, enabled: boolean) => {
    previewArgs(from, to, enabled);
    return { data: roster.data, isLoading: roster.isLoading, isError: roster.isError };
  },
}));

type Callbacks = { onSuccess?: (r: RotaReminderRequest) => void; onError?: (e: Error) => void };

// Wednesday 21 October 2026, 13:30 in Colombo. The next working day is Thursday
// 22 October and the weekend is Saturday 24 and Sunday 25 October.
const NOW = new Date("2026-10-21T08:00:00Z");

function assignment(name: string, shiftCode: string, rotaDate: string): ScheduleAssignment {
  const email = `${name.toLowerCase()}@example.test`;
  return {
    id: `${name}-${shiftCode}-${rotaDate}`,
    engineer: { userId: `u-${name}`, name, email, isLead: false },
    teamKey: "cre",
    shiftCode,
    rotaDate,
    startsAt: `${rotaDate}T00:00:00Z`,
    endsAt: `${rotaDate}T08:00:00Z`,
    isOnCall: false,
    source: "manual",
  };
}

const SAT = "2026-10-24";
const SUN = "2026-10-25";
const THU = "2026-10-22";

const ROSTERED: ScheduleAssignment[] = [
  assignment("Ann", "CRE_WEEKEND", SAT),
  assignment("Ben", "CRE_WEEKEND", SAT),
  assignment("Cid", "CRE_WEEKEND", SAT),
  assignment("Dee", "CRE_WEEKEND_NIGHT", SAT),
  assignment("Ann", "CRE_WEEKEND", SUN),
  assignment("Ben", "CRE_WEEKEND", SUN),
  assignment("Dee", "CRE_WEEKEND_NIGHT", SUN),
  assignment("Eve", "CRE_WEEKEND_NIGHT_OC", SUN),
  // Not a rotation the weekend reminder is about, so not counted.
  assignment("Zed", "CRE_MORNING", SAT),
  assignment("Fay", "CRE_MORNING", THU),
  assignment("Gus", "CRE_EVENING", THU),
  assignment("Hal", "CRE_EVENING", THU),
];

function setRoster(assignments: ScheduleAssignment[]): void {
  roster.data = { assignments, count: assignments.length };
}

function request(over: Partial<RotaReminderRequest>): RotaReminderRequest {
  return {
    id: "r1",
    family: "CRE",
    kind: "weekend",
    status: "QUEUED",
    requestedBy: "ann@example.com",
    requestedOn: "2026-10-21T05:00:00Z",
    ...over,
  };
}

const when = (iso: string): string => new Date(iso).toLocaleString();

const sendButton = () => screen.getByRole("button", { name: "Send now" });
const pickWeekend = (): void => {
  fireEvent.click(screen.getByRole("radio", { name: "The weekend" }));
};

beforeEach(() => {
  // Only the clock is fake, so nothing else waits on a timer.
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(NOW);
  mutate.mockReset();
  previewArgs.mockReset();
  sending.pending = false;
  requests.data = [];
  roster.isLoading = false;
  roster.isError = false;
  setRoster(ROSTERED);
});

afterEach(() => {
  vi.useRealTimers();
});

describe("RotaReminderSendNow", () => {
  it("offers the next working day and the weekend, each with its dates", () => {
    render(<RotaReminderSendNow />);
    expect(screen.getByRole("heading", { name: "Send now" })).toBeInTheDocument();
    expect(
      screen.getByText(
        "Sends the reminders and calendar invitations now, whatever the on/off switches above say. They go out at the scheduled task's next run, within the hour.",
      ),
    ).toBeInTheDocument();

    expect(screen.getByRole("group", { name: "What to send" })).toBeInTheDocument();
    const weekday = screen.getByRole("radio", { name: "Next working day (Morning and Evening)" });
    const weekend = screen.getByRole("radio", { name: "The weekend" });
    expect(weekday).toBeChecked();
    expect(weekend).not.toBeChecked();
    expect(weekday).toHaveAccessibleDescription("Thursday, 22 October");
    expect(weekend).toHaveAccessibleDescription("Saturday, 24 and Sunday, 25 October");
  });

  it("previews the next working day's rotations, with nobody on a rotation said so", () => {
    render(<RotaReminderSendNow />);
    expect(previewArgs).toHaveBeenLastCalledWith(THU, THU, true);

    const day = within(screen.getByRole("group", { name: "Thursday, 22 October" }));
    expect(day.getByText("Morning (1): Fay")).toBeInTheDocument();
    expect(day.getByText("Morning on-call: nobody")).toBeInTheDocument();
    expect(day.getByText("Evening (2): Gus, Hal")).toBeInTheDocument();
    expect(screen.getByText("People on leave are left out.")).toBeInTheDocument();
    expect(
      screen.queryByText(/^Sent even if the weekend is not fully marked/),
    ).not.toBeInTheDocument();
  });

  it("previews the weekend by day and rotation, and counts each person once", () => {
    render(<RotaReminderSendNow />);
    pickWeekend();
    expect(previewArgs).toHaveBeenLastCalledWith(SAT, SUN, true);

    const sat = within(screen.getByRole("group", { name: "Saturday, 24 October" }));
    expect(sat.getByText("LK Weekend (3): Ann, Ben, Cid")).toBeInTheDocument();
    expect(sat.getByText("Americas Weekend (1): Dee")).toBeInTheDocument();
    expect(sat.getByText("LK On-call: nobody")).toBeInTheDocument();

    const sun = within(screen.getByRole("group", { name: "Sunday, 25 October" }));
    expect(sun.getByText("LK Weekend (2): Ann, Ben")).toBeInTheDocument();
    expect(sun.getByText("Americas Weekend (1): Dee")).toBeInTheDocument();
    expect(sun.getByText("LK On-call (1): Eve")).toBeInTheDocument();

    expect(
      screen.getByText(
        "Sent even if the weekend is not fully marked: what is missing is listed and the CRE rota admins are told. Sending again updates the same calendar events.",
      ),
    ).toBeInTheDocument();

    // Ann, Ben, Cid, Dee and Eve: five people, though some are listed twice.
    fireEvent.click(sendButton());
    expect(screen.getByText("Email 5 people now?")).toBeInTheDocument();
  });

  it("says nobody is rostered, and does not let anything be sent", () => {
    setRoster([]);
    render(<RotaReminderSendNow />);
    expect(screen.getByText("Nobody is on these rotations, so nothing would be sent.")).toBeInTheDocument();
    expect(sendButton()).toBeDisabled();
  });

  it("says it is loading the rota, and when it could not", () => {
    roster.data = undefined;
    roster.isLoading = true;
    const { unmount } = render(<RotaReminderSendNow />);
    expect(screen.getByText("Loading who is on the rota…")).toBeInTheDocument();
    expect(sendButton()).toBeDisabled();
    unmount();

    roster.isLoading = false;
    roster.isError = true;
    render(<RotaReminderSendNow />);
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load the rota for the preview.");
    expect(sendButton()).toBeDisabled();
  });

  it("asks twice: nothing is sent until Yes, send", () => {
    render(<RotaReminderSendNow />);
    pickWeekend();
    fireEvent.click(sendButton());
    expect(mutate).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Yes, send" })).toBeEnabled();
    expect(screen.queryByRole("button", { name: "Send now" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Yes, send" }));
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(mutate.mock.calls[0]?.[0]).toEqual({ kind: "weekend" });
  });

  it("sends the next working day's reminders as weekday", () => {
    render(<RotaReminderSendNow />);
    fireEvent.click(sendButton());
    expect(screen.getByText("Email 3 people now?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Yes, send" }));
    expect(mutate.mock.calls[0]?.[0]).toEqual({ kind: "weekday" });
  });

  it("does not send on Cancel, and offers Send now again", () => {
    render(<RotaReminderSendNow />);
    pickWeekend();
    fireEvent.click(sendButton());
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(mutate).not.toHaveBeenCalled();
    expect(screen.queryByText(/^Email \d+ (people|person) now\?$/)).not.toBeInTheDocument();
    expect(sendButton()).toBeEnabled();
  });

  it("says it is queued once the ask is taken", () => {
    mutate.mockImplementation((_body: unknown, opts?: Callbacks) => opts?.onSuccess?.(request({})));
    render(<RotaReminderSendNow />);
    pickWeekend();
    fireEvent.click(sendButton());
    fireEvent.click(screen.getByRole("button", { name: "Yes, send" }));

    expect(screen.getByText("Queued. It goes out at the scheduled task's next run.")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByText(/^Email \d+ (people|person) now\?$/)).not.toBeInTheDocument();
  });

  it("shows the server's words when it refuses", () => {
    mutate.mockImplementation((_body: unknown, opts?: Callbacks) =>
      opts?.onError?.(new Error("a weekend reminder was asked for less than 10 minutes ago")),
    );
    render(<RotaReminderSendNow />);
    pickWeekend();
    fireEvent.click(sendButton());
    fireEvent.click(screen.getByRole("button", { name: "Yes, send" }));

    expect(screen.getByRole("alert")).toHaveTextContent("a weekend reminder was asked for less than 10 minutes ago");
    expect(screen.queryByText(/^Queued\./)).not.toBeInTheDocument();
    expect(sendButton()).toBeEnabled();
  });

  it("holds Send now, and says why, while a reminder of that kind is waiting", () => {
    requests.data = [request({ kind: "weekend", status: "QUEUED" })];
    render(<RotaReminderSendNow />);

    // The weekday reminder is not the one waiting.
    expect(sendButton()).toBeEnabled();
    expect(screen.queryByText(/is already waiting to be sent\.$/)).not.toBeInTheDocument();

    pickWeekend();
    expect(sendButton()).toBeDisabled();
    expect(screen.getByText("A weekend reminder is already waiting to be sent.")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("radio", { name: "Next working day (Morning and Evening)" }));
    expect(sendButton()).toBeEnabled();
  });

  it("holds Send now while a reminder of that kind is being sent", () => {
    requests.data = [request({ kind: "weekday", status: "SENDING" })];
    render(<RotaReminderSendNow />);
    expect(sendButton()).toBeDisabled();
    expect(screen.getByText("A weekday reminder is already waiting to be sent.")).toBeInTheDocument();
  });

  it("holds Send now while the ask is on its way", () => {
    sending.pending = true;
    render(<RotaReminderSendNow />);
    expect(sendButton()).toBeDisabled();
  });

  it("lists recent asks with where each one is", () => {
    requests.data = [
      request({ id: "a", kind: "weekend", status: "QUEUED", requestedBy: "ann@example.com" }),
      request({ id: "b", kind: "weekday", status: "SENDING", requestedBy: "ben@example.com" }),
      request({
        id: "c",
        kind: "weekend",
        status: "SENT",
        requestedBy: "cid@example.com",
        handledOn: "2026-10-20T10:00:00Z",
        result: "12 emails sent",
      }),
      request({ id: "d", kind: "weekday", status: "SENT", requestedBy: "dee@example.com", handledOn: "2026-10-19T10:00:00Z" }),
      request({ id: "e", kind: "weekday", status: "FAILED", requestedBy: "eve@example.com", result: "mail server down" }),
    ];
    render(<RotaReminderSendNow />);

    expect(screen.getByRole("heading", { name: "Recent" })).toBeInTheDocument();
    const list = within(screen.getByRole("list", { name: "Recent" }));
    const asked = when("2026-10-21T05:00:00Z");
    expect(list.getByText(`Weekend · asked by ann@example.com on ${asked} · Waiting to be sent`)).toBeInTheDocument();
    expect(list.getByText(`Weekday · asked by ben@example.com on ${asked} · Sending…`)).toBeInTheDocument();
    expect(
      list.getByText(
        `Weekend · asked by cid@example.com on ${asked} · Sent on ${when("2026-10-20T10:00:00Z")}: 12 emails sent`,
      ),
    ).toBeInTheDocument();
    expect(
      list.getByText(`Weekday · asked by dee@example.com on ${asked} · Sent on ${when("2026-10-19T10:00:00Z")}`),
    ).toBeInTheDocument();
    expect(
      list.getByText(`Weekday · asked by eve@example.com on ${asked} · Failed: mail server down`),
    ).toBeInTheDocument();
  });

  it("shows no Recent list when nothing was asked for", () => {
    render(<RotaReminderSendNow />);
    expect(screen.queryByRole("heading", { name: "Recent" })).not.toBeInTheDocument();
  });
});
