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

import {
  useEffect,
  useId,
  useRef,
  useState,
  type FormEvent,
  type JSX,
  type KeyboardEvent,
  type RefObject,
} from "react";
import {
  useCaseAlertSettings,
  useDeleteCaseAlertRoom,
  useDeleteCaseAlertRoute,
  useSaveCaseAlertRoom,
  useSaveCaseAlertRoute,
  useSaveCaseAlertValues,
  useSetCaseAlertWebhook,
  useTestCaseAlertRoom,
} from "../api/useCaseAlertSettings";
import type {
  CaseAlertChannel,
  CaseAlertChatRoom,
  CaseAlertGroupRoute,
  CaseAlertLadder,
  CaseAlertRouteOptions,
  CaseAlertSettings as Settings,
  CaseAlertTeamOption,
  PagingChainCode,
} from "../types";
import {
  channelWords,
  teamChannelsUnder,
  valuesToCopy,
} from "../utils/caseAlertLadderKeys";
import CaseAlertLadderSettings from "./CaseAlertLadderSettings";
import CaseAlertModal from "./CaseAlertModal";
import CaseAlertSection from "./CaseAlertSection";

interface ChainSettings {
  /** The chain as the picker names it. */
  label: string;
  ladder: CaseAlertLadder;
}

/** The chains with settings here. SME has none yet. Both SRE chains show
 *  the one SRE ladder, so every SRE team's chat routing. */
const CHAIN_SETTINGS: Partial<Record<PagingChainCode, ChainSettings>> = {
  CRE: { label: "CRE", ladder: "CRE" },
  SRE_SAAS: { label: "SaaS SRE", ladder: "SRE" },
  SRE_IAAS: { label: "IaaS SRE", ladder: "SRE" },
};

/** What a new room starts as, as the alert service's file has it today: its
 *  label, and whether it also gets the incidents with no SRE team (the SaaS
 *  teams do), so adding one never quietly changes where those go. Keyed by
 *  ladder, then team key ("" for the shared space). */
const ROOM_DEFAULTS: Record<
  CaseAlertLadder,
  Record<string, { label: string; teamless?: boolean }>
> = {
  CRE: { "": { label: "CRE Case Alerts" } },
  SRE: {
    "": { label: "SRE Case Alerts" },
    apollo: { label: "Apollo SRE", teamless: true },
    artemis: { label: "Artemis SRE", teamless: true },
    iaas: { label: "IaaS SRE" },
  },
};

/** The value the team select uses for the ladder's shared space. */
const SHARED = "*shared";

const WEBHOOK_PREFIX = "https://chat.googleapis.com/";

function when(iso: string | undefined): string {
  return iso ? new Date(iso).toLocaleString() : "";
}

/** What the alert service is doing with these settings, in one line. It is
 *  what the service last reported at start-up: switched back off, it no
 *  longer asks or reports, so the time is part of the answer. */
function statusLine(s: Settings): { tone: "info" | "warn"; text: string } {
  const r = s.revision;
  const asOf = r.appliedAt
    ? ` (as the alert service reported at ${when(r.appliedAt)})`
    : "";
  switch (r.appliedMode) {
    case "db":
      return r.applied === r.saved
        ? { tone: "info", text: `In use by the alert service${asOf}.` }
        : {
            tone: "warn",
            text: `Saved. Changes apply the next time the alert service restarts${asOf}.`,
          };
    case "shadow":
      return {
        tone: "warn",
        text: `Saved, not in use yet. The alert service is comparing these with its own settings, but still uses its own${asOf}.`,
      };
    default:
      return {
        tone: "warn",
        text: "Saved, not in use yet. The alert service still uses its own file and env settings until it is switched to these.",
      };
  }
}

interface Props {
  /** The chain on screen; its ladder's settings are shown. */
  chain: PagingChainCode;
}

/**
 * The Case Alerts settings for the chain on screen, under the tab's Settings
 * switch: a header saying what the alert service does with them, each team's
 * Google Chat space and how it is alerted, which team other CSM Portal teams'
 * alerts go to, and the rest of the ladder's paging file
 * (CaseAlertLadderSettings). Shown only to the people the server lets see
 * them (a CSM admin, portal admin, rota admin or head). What may be changed
 * is the server's answer too (canManage), and checked again on every write.
 *
 * A webhook URL is pasted once and never shown again: the page only ever
 * holds a hint. A room or route not saved here leaves the alert service on
 * its own setting for it.
 */
export default function CaseAlertSettings({
  chain,
}: Props): JSX.Element | null {
  const settings = useCaseAlertSettings();
  const [error, setError] = useState<string | null>(null);
  const [savedWhat, setSavedWhat] = useState<string | null>(null);
  const cfg = CHAIN_SETTINGS[chain];
  const s = settings.data;
  if (!cfg || !s || !s.canManage.view) return null;

  const ladder = cfg.ladder;
  const canRooms = s.canManage.roomLadders.includes(ladder);
  const canRoutes = s.canManage.routeLadders.includes(ladder);
  const rooms = s.rooms.filter((r) => r.ladder === ladder);
  const routes = s.routes.filter((r) => r.ladder === ladder);
  const status = statusLine(s);
  // The ladder's own channel is the file's; a team can only do less.
  const ladderChannel = s.running?.fileOnly?.[ladder]?.channel;
  /** A change went through: say so, in place of the last one. */
  const onSaved = (what: string) => {
    setError(null);
    setSavedWhat(what);
  };

  return (
    <section className="cas" aria-labelledby={`cas-h-${chain}`}>
      <div className="cas-head">
        <h3 id={`cas-h-${chain}`}>{cfg.label} settings</h3>
        {s.environment ? (
          <span className="cas-env">Environment: {s.environment}</span>
        ) : null}
      </div>
      <p className={`cas-status ${status.tone}`}>{status.text}</p>
      <div className="cas-live" aria-live="polite">
        {savedWhat ? (
          <p className="cas-status ok cas-done">
            <span>
              ✓ Saved: {savedWhat}. {whenUsed(s)}
            </span>
            <button
              type="button"
              className="cas-x"
              aria-label="Dismiss"
              onClick={() => setSavedWhat(null)}
            >
              ×
            </button>
          </p>
        ) : null}
      </div>
      {!canRooms ? (
        <p className="cas-sub cas-viewonly">
          You can view these. CSM admins and the {ladder} rota admin can change
          them.
        </p>
      ) : null}
      {canRooms ? (
        <SetupStep
          settings={s}
          ladder={ladder}
          onError={setError}
          onSaved={onSaved}
        />
      ) : null}
      {error ? (
        <div className="ra-err" role="alert">
          {error}
        </div>
      ) : null}

      <TeamChatRouting
        ladder={ladder}
        rooms={rooms}
        options={s.routeOptions}
        ladderChannel={ladderChannel}
        canEdit={canRooms}
        webhooksEnabled={s.webhooksEnabled}
        onError={setError}
        onSaved={onSaved}
      />

      <CaseAlertSection
        id={`${ladder}.redirects`}
        label={ROUTE_WORDS[ladder].title}
        summary={`${ROUTE_WORDS[ladder].title} · ${
          routes.length === 0
            ? "none"
            : `${routes.length} redirect${routes.length === 1 ? "" : "s"}`
        }`}
      >
        <p className="cas-sub">{ROUTE_WORDS[ladder].help}</p>
        <RouteTable
          ladder={ladder}
          routes={routes}
          options={s.routeOptions}
          rooms={rooms}
          ladderChannel={ladderChannel}
          canEdit={canRoutes}
          onError={setError}
          onSaved={onSaved}
        />
      </CaseAlertSection>

      <CaseAlertLadderSettings ladder={ladder} settings={s} onSaved={onSaved} />
    </section>
  );
}

/** When the alert service starts using a saved change. */
function whenUsed(s: Settings): string {
  return s.revision.appliedMode === "db"
    ? "The alert service uses it after its next restart."
    : "The alert service uses the saved settings once it is switched to them (db mode).";
}

/** A team's name for people: the shared space, else the Team Schedule's
 *  name for the team, else the room's own label. */
function roomTeamName(
  ladder: CaseAlertLadder,
  teamKey: string | undefined,
  options: CaseAlertRouteOptions | undefined,
  fallback: string,
): string {
  if (!teamKey) return `All ${ladder} teams (shared space)`;
  return options?.teams.find((t) => t.key === teamKey)?.name ?? fallback;
}

/** What a team's chat routing is called in the confirmation line. */
function routingWhat(teamName: string): string {
  return `${teamName.replace(/ \(shared space\)$/, "")} chat routing`;
}

interface TeamChatRoutingProps {
  ladder: CaseAlertLadder;
  /** The ladder's rooms saved here. */
  rooms: CaseAlertChatRoom[];
  options?: CaseAlertRouteOptions;
  ladderChannel?: string;
  canEdit: boolean;
  webhooksEnabled: boolean;
  onError: (msg: string | null) => void;
  onSaved: (what: string) => void;
}

/**
 * Team chat routing: which Google Chat space each team's alerts go to, and
 * whether the team gets chat cards, calls or both. Only routings saved here
 * are listed; every other team uses the alert service's file. Adding and
 * changing one goes through a dialog with an explicit Save; nothing in the
 * rows saves on change.
 */
function TeamChatRouting({
  ladder,
  rooms,
  options,
  ladderChannel,
  canEdit,
  webhooksEnabled,
  onError,
  onSaved,
}: TeamChatRoutingProps): JSX.Element {
  const [adding, setAdding] = useState(false);
  const addRef = useRef<HTMLButtonElement>(null);
  const ladderWords = channelWords(ladderChannel);
  const name = (r: CaseAlertChatRoom) =>
    roomTeamName(ladder, r.teamKey, options, r.label);
  // The shared space first, then the teams by name.
  const sorted = [...rooms].sort((a, b) =>
    !a.teamKey ? -1 : !b.teamKey ? 1 : name(a).localeCompare(name(b)),
  );
  // What can still be added: the shared space when it isn't saved, and (SRE)
  // each SRE team with no routing yet.
  const savedKeys = new Set(rooms.map((r) => r.teamKey ?? ""));
  const choices: { value: string; label: string }[] = [
    ...(savedKeys.has("")
      ? []
      : [{ value: SHARED, label: `All ${ladder} teams (shared space)` }]),
    ...(ladder === "SRE"
      ? (options?.teams ?? [])
          .filter((t) => t.family === "SRE" && !savedKeys.has(t.key))
          .map((t) => ({ value: t.key, label: t.name }))
      : []),
  ];
  const addBlocked =
    choices.length === 0
      ? "Every team already has a chat routing"
      : !webhooksEnabled
        ? "Webhook URLs can't be saved on this server yet"
        : undefined;

  return (
    <>
      <div className="cas-h4row">
        <h4>Team chat routing</h4>
        {canEdit && choices.length > 0 ? (
          <button
            ref={addRef}
            type="button"
            className="btn sm primary"
            disabled={addBlocked !== undefined}
            title={addBlocked}
            onClick={() => setAdding(true)}
          >
            + Add team chat routing
          </button>
        ) : null}
      </div>
      {canEdit && choices.length === 0 ? (
        // Said, not hidden in a disabled button's tooltip.
        <p className="cas-sub cas-note">
          Every {ladder} team already has a chat routing here. To change one,
          use its Edit; to start a team again, use More &gt; Remove.
        </p>
      ) : null}
      <p className="cas-sub">
        Which Google Chat space each team&apos;s alerts go to, and whether the
        team gets chat cards, calls or both.
        {ladderWords
          ? ` This ladder sends alerts by ${ladderWords.toLowerCase()} (set in the alert service's file). A team can do less, never more.`
          : ""}
      </p>
      {!webhooksEnabled ? (
        <p className="cas-sub cas-warn">
          Webhook URLs can&apos;t be saved on this server yet: its encryption
          key is not set.
        </p>
      ) : null}
      {sorted.length === 0 ? (
        <p className="cas-sub cas-empty">
          No team chat routing saved here yet. Alerts go to the chat spaces in
          the alert service&apos;s file.
        </p>
      ) : (
        <table className="ra-tbl cas-rooms" aria-label="Team chat routing">
          <thead>
            <tr>
              <th>Team</th>
              <th>Google Chat space</th>
              <th>Alerts by</th>
              {canEdit ? <th aria-label="Actions" /> : null}
            </tr>
          </thead>
          <tbody>
            {sorted.map((room) => (
              <ChatRow
                key={room.id}
                ladder={ladder}
                room={room}
                teamName={name(room)}
                ladderChannel={ladderChannel}
                canEdit={canEdit}
                webhooksEnabled={webhooksEnabled}
                onError={onError}
                onSaved={onSaved}
              />
            ))}
          </tbody>
        </table>
      )}
      {adding ? (
        <RoomDialog
          ladder={ladder}
          choices={choices}
          options={options}
          ladderChannel={ladderChannel}
          webhooksEnabled={webhooksEnabled}
          returnFocus={addRef}
          onClose={() => setAdding(false)}
          onSaved={onSaved}
        />
      ) : null}
    </>
  );
}

interface ChatRowProps {
  ladder: CaseAlertLadder;
  room: CaseAlertChatRoom;
  teamName: string;
  ladderChannel?: string;
  canEdit: boolean;
  webhooksEnabled: boolean;
  onError: (msg: string | null) => void;
  onSaved: (what: string) => void;
}

/** One saved chat routing, read-only: changes go through Edit and its Save. */
function ChatRow({
  ladder,
  room,
  teamName,
  ladderChannel,
  canEdit,
  webhooksEnabled,
  onError,
  onSaved,
}: ChatRowProps): JSX.Element {
  const saveRoom = useSaveCaseAlertRoom();
  const test = useTestCaseAlertRoom();
  const remove = useDeleteCaseAlertRoom();
  const [editing, setEditing] = useState(false);
  const editRef = useRef<HTMLButtonElement>(null);
  const [confirm, setConfirm] = useState<"off" | "remove" | null>(null);
  const confirmRef = useRef<HTMLButtonElement>(null);
  // The menu that asked is gone: the question takes focus.
  useEffect(() => {
    if (confirm) confirmRef.current?.focus();
  }, [confirm]);
  const busy = saveRoom.isPending || test.isPending || remove.isPending;
  const fail = (e: Error) => onError(e.message || "That change did not save.");
  const what = routingWhat(teamName);
  const ladderWords = channelWords(ladderChannel);
  const alertsBy = room.teamKey
    ? room.channel
      ? channelWords(room.channel)
      : ladderWords
        ? `Same as the ladder (${ladderWords})`
        : "Same as the ladder"
    : (ladderWords ?? "—");

  /** Turns the room on or off, sending every field so nothing else moves. */
  const setEnabled = (enabled: boolean) => {
    onError(null);
    setConfirm(null);
    saveRoom.mutate(
      {
        ladder,
        teamKey: room.teamKey,
        label: room.label,
        enabled,
        teamless: room.teamless,
        ...(room.teamKey ? { channel: room.channel ?? "" } : {}),
      },
      {
        onError: fail,
        onSuccess: () => onSaved(`${what} turned ${enabled ? "on" : "off"}`),
      },
    );
  };

  return (
    <tr className={room.enabled ? undefined : "cas-off"}>
      <td>
        <b>{teamName}</b>
        <div className="cas-sub">
          {room.label}
          {room.teamless && room.teamKey
            ? " · also gets incidents with no SRE team"
            : ""}
          {!room.enabled ? <span className="cas-chip">Off</span> : null}
        </div>
      </td>
      <td>
        {room.hasWebhook ? (
          <>
            <code className="cas-hint">{room.webhookHint}</code>
            {room.lastTestStatus ? (
              <div className="cas-sub">
                Last test:{" "}
                <span className={`cas-test ${room.lastTestStatus}`}>
                  {room.lastTestStatus}
                </span>
                {room.lastTestAt ? ` · ${when(room.lastTestAt)}` : ""}
              </div>
            ) : null}
          </>
        ) : (
          <span className="cas-sub">
            No URL saved (uses the file&apos;s space)
          </span>
        )}
      </td>
      <td>
        <span>{alertsBy}</span>
        {!room.teamKey && ladderWords ? (
          <span className="cas-chip">set in the file</span>
        ) : null}
      </td>
      {canEdit ? (
        <td>
          <div className="cas-acts">
            {confirm ? (
              <>
                <span className="cas-sub">
                  {confirm === "off"
                    ? "Turn off? Its alerts use the file's space."
                    : "Remove? Its alerts use the file's space."}
                </span>
                <button
                  type="button"
                  className="btn sm danger"
                  disabled={busy}
                  ref={confirmRef}
                  onClick={() => {
                    if (confirm === "off") {
                      setEnabled(false);
                      return;
                    }
                    onError(null);
                    setConfirm(null);
                    remove.mutate(room.id, {
                      onError: fail,
                      onSuccess: () => onSaved(`${what} removed`),
                    });
                  }}
                >
                  {confirm === "off" ? "Turn off" : "Remove"}
                </button>
                <button
                  type="button"
                  className="btn sm"
                  onClick={() => setConfirm(null)}
                >
                  Cancel
                </button>
              </>
            ) : (
              <>
                <button
                  ref={editRef}
                  type="button"
                  className="btn sm"
                  disabled={busy}
                  onClick={() => setEditing(true)}
                >
                  Edit
                </button>
                {room.hasWebhook ? (
                  <button
                    type="button"
                    className="btn sm"
                    disabled={busy || !webhooksEnabled}
                    onClick={() => {
                      onError(null);
                      test.mutate(room.id, { onError: fail });
                    }}
                  >
                    {test.isPending ? "Sending…" : "Test"}
                  </button>
                ) : null}
                <MoreMenu
                  disabled={busy}
                  items={[
                    room.enabled
                      ? {
                          label: "Turn off",
                          onSelect: () => setConfirm("off"),
                        }
                      : {
                          label: "Turn on",
                          onSelect: () => setEnabled(true),
                        },
                    { label: "Remove", onSelect: () => setConfirm("remove") },
                  ]}
                />
              </>
            )}
          </div>
          {editing ? (
            <RoomDialog
              ladder={ladder}
              room={room}
              teamName={teamName}
              ladderChannel={ladderChannel}
              webhooksEnabled={webhooksEnabled}
              returnFocus={editRef}
              onClose={() => setEditing(false)}
              onSaved={onSaved}
            />
          ) : null}
        </td>
      ) : null}
    </tr>
  );
}

interface RoomDialogProps {
  ladder: CaseAlertLadder;
  /** Editing this room; absent when adding one. */
  room?: CaseAlertChatRoom;
  /** The room's team, as the row names it (edit only). */
  teamName?: string;
  /** The teams that can still be added (add only). */
  choices?: { value: string; label: string }[];
  options?: CaseAlertRouteOptions;
  ladderChannel?: string;
  webhooksEnabled: boolean;
  returnFocus: RefObject<HTMLButtonElement | null>;
  onClose: () => void;
  onSaved: (what: string) => void;
}

/**
 * Add a team chat routing, or change one: the team, its space's webhook URL,
 * how it is alerted and whether it also gets incidents with no SRE team.
 * Save sends the room with every field, then the URL when one was typed.
 * Errors stay in the dialog.
 */
function RoomDialog({
  ladder,
  room,
  teamName,
  choices = [],
  options,
  ladderChannel,
  webhooksEnabled,
  returnFocus,
  onClose,
  onSaved,
}: RoomDialogProps): JSX.Element {
  const saveRoom = useSaveCaseAlertRoom();
  const setWebhook = useSetCaseAlertWebhook();
  const ids = useId();
  const titleId = `${ids}-title`;
  const defaults = ROOM_DEFAULTS[ladder];
  const [team, setTeam] = useState(
    room ? (room.teamKey ?? SHARED) : (choices[0]?.value ?? SHARED),
  );
  const teamKey = team === SHARED ? undefined : team;
  const [url, setUrl] = useState("");
  const [channel, setChannel] = useState<CaseAlertChannel | "">(
    room?.channel ?? "",
  );
  const [teamless, setTeamless] = useState(
    room ? room.teamless : Boolean(defaults[teamKey ?? ""]?.teamless),
  );
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const ladderWords = channelWords(ladderChannel);
  const allowed = teamChannelsUnder(ladderChannel);
  const name =
    teamName ??
    roomTeamName(
      ladder,
      teamKey,
      options,
      choices.find((c) => c.value === team)?.label ?? team,
    );

  const pickTeam = (value: string) => {
    setTeam(value);
    // A new team starts as the file has it today.
    setTeamless(Boolean(defaults[value === SHARED ? "" : value]?.teamless));
    setChannel("");
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setError(null);
    const value = url.trim();
    if (!room && !value) {
      setError("Paste the space's Google Chat webhook URL.");
      return;
    }
    if (value && !value.startsWith(WEBHOOK_PREFIX)) {
      setError(
        `That isn't a Google Chat webhook URL. It starts with ${WEBHOOK_PREFIX}`,
      );
      return;
    }
    const label =
      room?.label ?? defaults[teamKey ?? ""]?.label ?? `${name} ${ladder}`;
    setBusy(true);
    try {
      const saved = await saveRoom.mutateAsync({
        ladder,
        teamKey,
        label,
        enabled: room?.enabled ?? true,
        teamless: teamKey ? teamless : (room?.teamless ?? false),
        ...(teamKey ? { channel } : {}),
      });
      if (value) await setWebhook.mutateAsync({ roomId: saved.id, url: value });
      onSaved(routingWhat(name));
      onClose();
    } catch (err) {
      setError((err as Error).message || "That change did not save.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <CaseAlertModal
      titleId={titleId}
      returnFocus={returnFocus}
      onClose={onClose}
    >
      <form className="cas-edit" noValidate onSubmit={(e) => void submit(e)}>
        <h3 id={titleId}>
          {room
            ? `Edit ${name.replace(/ \(shared space\)$/, "")} chat routing`
            : "Add team chat routing"}
        </h3>
        {room ? (
          <p className="cp-help">
            Team: <b>{name}</b>
          </p>
        ) : (
          <label>
            Team
            <select value={team} onChange={(e) => pickTeam(e.target.value)}>
              {choices.map((c) => (
                <option key={c.value} value={c.value}>
                  {c.label}
                </option>
              ))}
            </select>
          </label>
        )}
        <label>
          Google Chat webhook URL
          <input
            type="password"
            autoComplete="off"
            spellCheck={false}
            value={url}
            disabled={!webhooksEnabled}
            onChange={(e) => setUrl(e.target.value)}
            placeholder={`${WEBHOOK_PREFIX}v1/spaces/…`}
          />
        </label>
        <p className="cp-help">
          {room?.hasWebhook
            ? `Leave empty to keep the saved URL (${room.webhookHint ?? "hidden"}).`
            : "Copy it from the space: Apps & integrations, then Webhooks. It's stored encrypted and never shown again."}
        </p>
        {teamKey ? (
          <>
            <label>
              Alerts by
              <select
                value={channel}
                onChange={(e) =>
                  setChannel(e.target.value as CaseAlertChannel | "")
                }
              >
                <option value="">
                  {ladderWords
                    ? `Same as the ladder (${ladderWords})`
                    : "Same as the ladder"}
                </option>
                {allowed.map((c) => (
                  <option key={c} value={c}>
                    {channelWords(c)}
                  </option>
                ))}
                {/* Saved before the ladder's channel narrowed: kept as is. */}
                {channel && !allowed.includes(channel) ? (
                  <option value={channel}>
                    {channelWords(channel)} (more than the ladder)
                  </option>
                ) : null}
              </select>
            </label>
            {ladder === "SRE" ? (
              <label className="cas-check">
                <input
                  type="checkbox"
                  checked={teamless}
                  onChange={(e) => setTeamless(e.target.checked)}
                />
                Also gets incidents with no SRE team
              </label>
            ) : null}
          </>
        ) : null}
        {error ? (
          <p className="cp-err" role="alert">
            {error}
          </p>
        ) : null}
        <div className="cp-modal-actions">
          <button type="button" className="btn sm" onClick={onClose}>
            Cancel
          </button>
          <button
            type="submit"
            className="btn sm primary"
            aria-disabled={busy || undefined}
          >
            {busy ? "Saving…" : "Save"}
          </button>
        </div>
      </form>
    </CaseAlertModal>
  );
}

interface SetupStepProps {
  settings: Settings;
  ladder: CaseAlertLadder;
  onError: (msg: string | null) => void;
  onSaved: (what: string) => void;
}

/** The step left before moving off the file: the values the alert service
 *  runs with from its file that aren't saved here yet, saved in one request
 *  after a second click. Gone once there are none. */
function SetupStep({
  settings,
  ladder,
  onError,
  onSaved,
}: SetupStepProps): JSX.Element | null {
  const save = useSaveCaseAlertValues();
  const [confirm, setConfirm] = useState(false);
  const values = valuesToCopy(settings, ladder);
  const count = Object.keys(values).length;
  if (count === 0) return null;
  const word = count === 1 ? "setting" : "settings";

  return (
    <div className="cas-status warn cas-setup">
      <span>
        {count} {word} still {count === 1 ? "comes" : "come"} from the alert
        service&apos;s file.
      </span>
      {confirm ? (
        <>
          <span>
            Save {count} {word} here?
          </span>
          <button
            type="button"
            className="btn sm primary"
            disabled={save.isPending}
            onClick={() => {
              onError(null);
              setConfirm(false);
              save.mutate(
                { ladder, values },
                {
                  onError: (e: Error) =>
                    onError(e.message || "Those settings did not save."),
                  onSuccess: () => onSaved(`${count} ${word} from the file`),
                },
              );
            }}
          >
            Save {count} {word}
          </button>
          <button
            type="button"
            className="btn sm"
            onClick={() => setConfirm(false)}
          >
            Cancel
          </button>
        </>
      ) : (
        <button
          type="button"
          className="btn sm"
          disabled={save.isPending}
          onClick={() => setConfirm(true)}
        >
          Save them here
        </button>
      )}
    </div>
  );
}

interface MoreMenuProps {
  disabled: boolean;
  items: { label: string; onSelect: () => void }[];
}

/** A row's less common actions behind one button. Opens a short list; Esc
 *  or a click outside closes it, and focus goes back to the button. Arrow
 *  keys move through the list. */
function MoreMenu({ disabled, items }: MoreMenuProps): JSX.Element {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    wrapRef.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
    const onDown = (e: MouseEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (!open) return;
    if (e.key === "Escape") {
      e.stopPropagation();
      setOpen(false);
      buttonRef.current?.focus();
      return;
    }
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    e.preventDefault();
    const list = [
      ...(wrapRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ??
        []),
    ];
    const at = list.indexOf(document.activeElement as HTMLElement);
    const next =
      e.key === "ArrowDown"
        ? (at + 1) % list.length
        : (at - 1 + list.length) % list.length;
    list[next]?.focus();
  };

  return (
    <div className="cas-more" ref={wrapRef} onKeyDown={onKeyDown}>
      <button
        ref={buttonRef}
        type="button"
        className="btn sm"
        aria-haspopup="menu"
        aria-expanded={open}
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
      >
        More <span aria-hidden="true">▾</span>
      </button>
      {open ? (
        <ul className="cas-menu" role="menu">
          {items.map((item) => (
            <li key={item.label} role="none">
              <button
                type="button"
                role="menuitem"
                onClick={() => {
                  setOpen(false);
                  // Back to the button first; a question the item opens
                  // takes focus from there.
                  buttonRef.current?.focus();
                  item.onSelect();
                }}
              >
                {item.label}
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

interface RouteTableProps {
  ladder: CaseAlertLadder;
  routes: CaseAlertGroupRoute[];
  options?: CaseAlertRouteOptions;
  /** The ladder's saved rooms, to say where a team's alerts end up. */
  rooms: CaseAlertChatRoom[];
  /** The ladder's own channel from the file, when reported. */
  ladderChannel?: string;
  canEdit: boolean;
  onError: (msg: string | null) => void;
  onSaved: (what: string) => void;
}

/** How each ladder's redirects are put to the reader. An SRE incident and a
 *  CRE case both carry the CSM Portal team they are assigned to; a redirect
 *  sends its alerts to one of the ladder's own teams instead. */
const ROUTE_WORDS: Record<
  CaseAlertLadder,
  { title: string; help: string; when: string; empty: string }
> = {
  SRE: {
    title: "Send an incident's alerts to another SRE team",
    help:
      "An incident is alerted to the SRE team it's assigned to in CSM Portal. If it's assigned to a different " +
      "team, such as an SME team, choose which SRE team should get its alerts: that team's chat space gets the " +
      "card, and its people on call get the calls, as set in Team chat routing above.",
    when: "When an incident is assigned to",
    empty:
      "No redirects. Each incident is alerted to the SRE team it's assigned to.",
  },
  CRE: {
    title: "Send a case's alerts to another CRE team",
    help:
      "A case is alerted to the CRE team it's assigned to in CSM Portal. If a team there doesn't match a Team " +
      "Schedule team, choose which CRE team should get its alerts.",
    when: "When a case is assigned to",
    empty:
      "No redirects. Each case is alerted to the CRE team it's assigned to.",
  },
};

/** The CSM Portal teams a ladder's routes can be for: a CRE case's team, or
 *  an SRE incident's SRE or SME team. */
const ROUTE_GROUP_FAMILIES: Record<
  CaseAlertLadder,
  CaseAlertTeamOption["family"][]
> = {
  CRE: ["CRE"],
  SRE: ["SRE", "SME"],
};

function RouteTable({
  ladder,
  routes,
  options,
  rooms,
  ladderChannel,
  canEdit,
  onError,
  onSaved,
}: RouteTableProps): JSX.Element {
  const saveRoute = useSaveCaseAlertRoute();
  const removeRoute = useDeleteCaseAlertRoute();
  const words = ROUTE_WORDS[ladder];
  const [group, setGroup] = useState("");
  // "team:<key>" or "rota:<code>": one list, the ladder's teams first.
  const [target, setTarget] = useState("");
  const [confirmId, setConfirmId] = useState<string | null>(null);
  const busy = saveRoute.isPending || removeRoute.isPending;
  const fail = (e: Error) => onError(e.message || "That change did not save.");
  const families = ROUTE_GROUP_FAMILIES[ladder];
  const groups = (options?.groups ?? []).filter((g) =>
    families.includes(g.family),
  );
  const teams = (options?.teams ?? []).filter((t) => t.family === ladder);
  const rotas = options?.rotas ?? [];
  // A saved route holds the team's name in lower case; show it as the team
  // is named, and a target by its name too.
  const groupLabel = (name: string) =>
    groups.find((g) => g.name.toLowerCase() === name.toLowerCase())?.name ??
    name;
  const teamLabel = (key: string) =>
    teams.find((t) => t.key === key)?.name ?? key;
  const rotaLabel = (code: string) => {
    const r = rotas.find((x) => x.code === code);
    return r && r.label !== code ? r.label : code;
  };
  /** Where a team's alerts end up, from Team chat routing: its chat space and
   *  how they go out. */
  const teamOutcome = (key: string) => {
    const room = rooms.find((r) => r.teamKey === key);
    const channel = room?.channel || ladderChannel;
    const by = channelWords(channel) ?? "As the ladder";
    // Where the card goes only matters when the team gets chat cards.
    if (channel === "call" || channel === "log") return by;
    const space = room?.hasWebhook
      ? `in the ${room.label} space`
      : "in its own chat space";
    return `${by} · ${space}`;
  };
  /** A redirect from a team to itself: it changes nothing. */
  // The alert service matches an incident's team by name against the Team
  // Schedule's key, so a redirect is only needless when the name already is
  // the key (a team named "Apollo" with key apollo). A team named "Apollo SRE
  // Group" with key apollo does need it, though both read "Apollo...".
  const sameTeam = (r: CaseAlertGroupRoute) =>
    !!r.teamKey && r.groupName.trim().toLowerCase() === r.teamKey.toLowerCase();

  const add = (e: FormEvent) => {
    e.preventDefault();
    onError(null);
    const [kind, value] = target.split(":");
    saveRoute.mutate(
      {
        ladder,
        groupName: group,
        ...(kind === "team" ? { teamKey: value } : { rotaCode: value }),
      },
      {
        onError: fail,
        onSuccess: () => {
          onSaved(
            `redirect from ${groupLabel(group)} to ${
              kind === "team"
                ? teamLabel(value)
                : `every team on ${rotaLabel(value)}`
            }`,
          );
          setGroup("");
          setTarget("");
        },
      },
    );
  };

  return (
    <>
      {routes.length === 0 ? (
        <p className="cas-sub cas-empty">{words.empty}</p>
      ) : (
        <table className="ra-tbl cas-routes">
          <thead>
            <tr>
              <th>Assigned to in CSM Portal</th>
              <th>Alerts go to</th>
              {canEdit ? <th aria-label="Actions" /> : null}
            </tr>
          </thead>
          <tbody>
            {routes.map((r) => (
              <tr key={r.id} className={r.enabled ? undefined : "cas-off"}>
                <td>{groupLabel(r.groupName)}</td>
                <td>
                  {r.teamKey ? (
                    <>
                      <b>{teamLabel(r.teamKey)}</b>
                      <div className="cas-sub">
                        {sameTeam(r)
                          ? "Same team, not needed: it is alerted as itself anyway"
                          : teamOutcome(r.teamKey)}
                      </div>
                    </>
                  ) : (
                    <>
                      <b>
                        Every team on the {rotaLabel(r.rotaCode ?? "")} rota
                      </b>
                      <div className="cas-sub">
                        Whoever is on call there gets the alerts
                      </div>
                    </>
                  )}
                </td>
                {canEdit ? (
                  <td>
                    <div className="cas-acts">
                      {confirmId === r.id ? (
                        <>
                          <button
                            type="button"
                            className="btn sm danger"
                            disabled={busy}
                            onClick={() => {
                              onError(null);
                              setConfirmId(null);
                              removeRoute.mutate(r.id, {
                                onError: fail,
                                onSuccess: () =>
                                  onSaved(
                                    `redirect from ${groupLabel(r.groupName)} removed`,
                                  ),
                              });
                            }}
                          >
                            Remove
                          </button>
                          <button
                            type="button"
                            className="btn sm"
                            onClick={() => setConfirmId(null)}
                          >
                            Cancel
                          </button>
                        </>
                      ) : (
                        <button
                          type="button"
                          className="btn sm"
                          disabled={busy}
                          onClick={() => setConfirmId(r.id)}
                        >
                          Remove
                        </button>
                      )}
                    </div>
                  </td>
                ) : null}
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {canEdit ? (
        <form className="cas-add cas-route-add" onSubmit={add}>
          <label>
            {words.when}
            <select value={group} onChange={(e) => setGroup(e.target.value)}>
              <option value="">Choose a team</option>
              {families.map((family) => {
                const inFamily = groups.filter((g) => g.family === family);
                return inFamily.length ? (
                  <optgroup key={family} label={`${family} teams`}>
                    {inFamily.map((g) => (
                      <option key={g.key} value={g.name}>
                        {g.name}
                      </option>
                    ))}
                  </optgroup>
                ) : null;
              })}
            </select>
          </label>
          <label>
            send its alerts to
            <select value={target} onChange={(e) => setTarget(e.target.value)}>
              <option value="">
                {ladder === "SRE" ? "Choose an SRE team" : "Choose a CRE team"}
              </option>
              <optgroup label={`${ladder} teams`}>
                {teams.map((t) => (
                  <option key={t.key} value={`team:${t.key}`}>
                    {t.name}
                  </option>
                ))}
              </optgroup>
              {ladder === "SRE" && rotas.length ? (
                <optgroup label="A whole rota">
                  {rotas.map((r) => (
                    <option key={r.code} value={`rota:${r.code}`}>
                      Every team on {rotaLabel(r.code)}
                    </option>
                  ))}
                </optgroup>
              ) : null}
            </select>
          </label>
          <button
            type="submit"
            className="btn sm"
            disabled={busy || !group || !target}
          >
            Add
          </button>
        </form>
      ) : null}
    </>
  );
}
