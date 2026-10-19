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
import { useId, useState, type JSX } from "react";
import { useReminderPreview, useRotaReminderRequests, useSendRotaReminder } from "../api/useRotaReminderRequests";
import type { RotaFamily, RotaReminderKind, RotaReminderRequest, ScheduleAssignment } from "../types";
import { describeDays, reminderDays } from "../utils/reminderDays";

const FAMILY: RotaFamily = "CRE";

interface Rotation {
  code: string;
  label: string;
}

/** The rotations each reminder is about, in the order they are listed. */
const ROTATIONS: Record<RotaReminderKind, readonly Rotation[]> = {
  weekday: [
    { code: "CRE_MORNING", label: "Morning" },
    { code: "CRE_MORNING_OC", label: "Morning on-call" },
    { code: "CRE_EVENING", label: "Evening" },
  ],
  weekend: [
    { code: "CRE_WEEKEND", label: "LK Weekend" },
    { code: "CRE_WEEKEND_NIGHT", label: "Americas Weekend" },
    { code: "CRE_WEEKEND_NIGHT_OC", label: "LK On-call" },
  ],
};

const OPTIONS: readonly { kind: RotaReminderKind; label: string }[] = [
  { kind: "weekday", label: "Next working day (Morning and Evening)" },
  { kind: "weekend", label: "The weekend" },
];

const KIND_LABEL: Record<RotaReminderKind, string> = { weekday: "Weekday", weekend: "Weekend" };

interface PreviewDay {
  date: string;
  rotations: { label: string; names: string[] }[];
}

/** One person, whoever lists them: by email, or by user id when there is none. */
function personKey(a: ScheduleAssignment): string {
  return (a.engineer.email || a.engineer.userId).toLowerCase();
}

/**
 * Who is on each rotation on each day, and how many different people that is
 * in all (one person on two rotations is one email). Assignments of any other
 * rotation are not what the reminder is about and are left out.
 */
function buildPreview(
  kind: RotaReminderKind,
  days: readonly string[],
  assignments: readonly ScheduleAssignment[],
): { days: PreviewDay[]; people: number } {
  const everyone = new Set<string>();
  const perDay = days.map((date): PreviewDay => ({
    date,
    rotations: ROTATIONS[kind].map(({ code, label }) => {
      const names = new Map<string, string>();
      for (const a of assignments) {
        if (a.shiftCode !== code || a.rotaDate.slice(0, 10) !== date) continue;
        const key = personKey(a);
        everyone.add(key);
        if (!names.has(key)) names.set(key, a.engineer.name || a.engineer.email);
      }
      return { label, names: [...names.values()].sort((x, y) => x.localeCompare(y)) };
    }),
  }));
  return { days: perDay, people: everyone.size };
}

function previewLine(r: { label: string; names: string[] }): string {
  return r.names.length === 0 ? `${r.label}: nobody` : `${r.label} (${r.names.length}): ${r.names.join(", ")}`;
}

function when(iso: string): string {
  return new Date(iso).toLocaleString();
}

function requestLine(r: RotaReminderRequest): string {
  const head = `${KIND_LABEL[r.kind]} · asked by ${r.requestedBy} on ${when(r.requestedOn)}`;
  switch (r.status) {
    case "QUEUED":
      return `${head} · Waiting to be sent`;
    case "SENDING":
      return `${head} · Sending…`;
    case "SENT":
      return `${head} · Sent${r.handledOn ? ` on ${when(r.handledOn)}` : ""}${r.result ? `: ${r.result}` : ""}`;
    case "FAILED":
      return `${head} · Failed${r.result ? `: ${r.result}` : ""}`;
  }
}

/**
 * The "Send now" part of the CRE rota reminders card: asks for the weekday or
 * the weekend reminders, and their calendar invitations, to go out now rather
 * than at their usual time. It shows who would be emailed first and asks once
 * more before it asks. The ask is queued: the scheduled task sends it at its
 * next run. The server decides who may ask and how often, and answers a
 * refusal in words, shown as it is.
 */
export default function RotaReminderSendNow(): JSX.Element {
  const uid = useId();
  const id = (part: string): string => `${uid}-${part}`;
  const headingId = id("h");

  const [kind, setKind] = useState<RotaReminderKind>("weekday");
  const [confirming, setConfirming] = useState(false);
  const [queued, setQueued] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const days = reminderDays(kind, new Date());
  const from = days[0] ?? "";
  const to = days[days.length - 1] ?? from;

  const requests = useRotaReminderRequests(FAMILY);
  const send = useSendRotaReminder(FAMILY);
  const roster = useReminderPreview(from, to, true);

  const recent = requests.data ?? [];
  const waiting = recent.some((r) => r.kind === kind && (r.status === "QUEUED" || r.status === "SENDING"));

  const preview = roster.data ? buildPreview(kind, days, roster.data.assignments) : null;
  const nobody = preview !== null && preview.people === 0;
  const canAsk = preview !== null && !nobody && !waiting && !send.isPending;

  const pick = (next: RotaReminderKind): void => {
    setKind(next);
    setConfirming(false);
    setQueued(false);
    setError(null);
  };

  const ask = (): void => {
    if (!canAsk) return;
    setError(null);
    setQueued(false);
    send.mutate(
      { kind },
      {
        onSuccess: () => {
          setConfirming(false);
          setQueued(true);
        },
        onError: (err) => {
          setConfirming(false);
          setError(err.message || "The reminders were not requested.");
        },
      },
    );
  };

  return (
    <section className="rrs-send" aria-labelledby={headingId}>
      <h4 id={headingId}>Send now</h4>
      <p className="cp-help">
        Sends the reminders and calendar invitations now, whatever the on/off switches above say. They go out at the
        scheduled task&apos;s next run, within the hour.
      </p>

      <fieldset className="rrs-kinds">
        <legend>What to send</legend>
        {OPTIONS.map((o) => (
          <div className="rrs-opt" key={o.kind}>
            <label className="cas-check" htmlFor={id(o.kind)}>
              <input
                id={id(o.kind)}
                type="radio"
                name={id("kind")}
                value={o.kind}
                checked={kind === o.kind}
                aria-describedby={id(`${o.kind}-when`)}
                onChange={() => pick(o.kind)}
              />
              {o.label}
            </label>
            <span className="rrs-when" id={id(`${o.kind}-when`)}>
              {describeDays(reminderDays(o.kind, new Date()))}
            </span>
          </div>
        ))}
      </fieldset>
      {kind === "weekend" ? (
        <p className="cp-help">
          Sent even if the weekend is not fully marked: what is missing is listed and the CRE rota admins are told.
          Sending again updates the same calendar events.
        </p>
      ) : null}

      {roster.isLoading ? <p className="cp-help">Loading who is on the rota…</p> : null}
      {roster.isError ? (
        <div className="ra-err" role="alert">
          Could not load the rota for the preview.
        </div>
      ) : null}
      {preview ? (
        <div className="rrs-days">
          {preview.days.map((d) => {
            const name = describeDays([d.date]);
            return (
              <div className="rrs-day" role="group" aria-label={name} key={d.date}>
                <div className="rrs-day-name">{name}</div>
                <ul className="rrs-lines">
                  {d.rotations.map((r) => (
                    <li key={r.label}>{previewLine(r)}</li>
                  ))}
                </ul>
              </div>
            );
          })}
          <p className="cp-help">People on leave are left out.</p>
        </div>
      ) : null}
      {nobody ? (
        <p className="cas-status info">Nobody is on these rotations, so nothing would be sent.</p>
      ) : null}

      {waiting ? <p className="cp-help">A {kind} reminder is already waiting to be sent.</p> : null}

      {confirming ? (
        <div className="rrs-acts" role="group" aria-labelledby={id("confirm")}>
          <span id={id("confirm")}>{(preview?.people ?? 0) === 1 ? "Email 1 person now?" : `Email ${preview?.people ?? 0} people now?`}</span>
          <button type="button" className="btn sm primary" disabled={!canAsk} onClick={ask}>
            Yes, send
          </button>
          <button type="button" className="btn sm" disabled={send.isPending} onClick={() => setConfirming(false)}>
            Cancel
          </button>
        </div>
      ) : (
        <div className="rrs-acts">
          <button
            type="button"
            className="btn sm primary"
            disabled={!canAsk}
            onClick={() => {
              setQueued(false);
              setError(null);
              setConfirming(true);
            }}
          >
            Send now
          </button>
        </div>
      )}

      {error ? (
        <div className="ra-err" role="alert">
          {error}
        </div>
      ) : null}
      <div aria-live="polite">
        {queued ? <p className="cas-status ok">Queued. It goes out at the scheduled task&apos;s next run.</p> : null}
      </div>

      {recent.length > 0 ? (
        <div>
          <h5 id={id("recent")}>Recent</h5>
          <ul className="rrs-recent" aria-labelledby={id("recent")}>
            {recent.map((r) => (
              <li key={r.id}>{requestLine(r)}</li>
            ))}
          </ul>
        </div>
      ) : null}
    </section>
  );
}
