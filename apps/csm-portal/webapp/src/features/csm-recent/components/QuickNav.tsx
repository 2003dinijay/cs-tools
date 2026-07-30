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

import {
  Box,
  ButtonBase,
  Form,
  InputBase,
  Modal,
  Paper,
  Typography,
} from "@wso2/oxygen-ui";
import { Search } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useRef, useState, type JSX } from "react";
import { useSearchParams } from "react-router";
import { useAsgardeo } from "@asgardeo/react";

import { navigableNavNodes } from "@config/featureFlags";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import {
  useRecentViews,
  type RecentView,
} from "@features/csm-recent/hooks/useRecentViews";
import { kindIcon } from "@features/csm-recent/kindMeta";
import QuickNavCaseCard from "@features/csm-recent/components/QuickNavCaseCard";
import QuickNavEntityCard from "@features/csm-recent/components/QuickNavEntityCard";
import QuickNavResultSkeleton from "@features/csm-recent/components/QuickNavResultSkeleton";
import SearchNoResultsIcon from "@components/empty-state/SearchNoResultsIcon";
import {
  QUICK_CASE_MIN_QUERY_LEN,
  useQuickCaseSearch,
  type QuickCaseHit,
} from "@features/csm-cases/api/useQuickCaseSearch";
import { caseIdLabel } from "@features/csm-cases/utils/caseIdentity";
import { useNavTransition } from "@hooks/useNavTransition";
import {
  QUICK_INCIDENT_MIN_QUERY_LEN,
  useQuickIncidentSearch,
} from "@features/csm-operations/api/useQuickIncidentSearch";
import {
  QUICK_CHANGE_REQUEST_MIN_QUERY_LEN,
  useQuickChangeRequestSearch,
} from "@features/csm-operations/api/useQuickChangeRequestSearch";
import {
  QUICK_PROBLEM_MIN_QUERY_LEN,
  useQuickProblemSearch,
} from "@features/csm-operations/api/useQuickProblemSearch";

type Section =
  | "Cases"
  | "Incidents"
  | "Change Requests"
  | "Problems"
  | "Pinned"
  | "Recents"
  | "Pages";

/** Minimal shape `QuickNavEntityCard` needs, shared by incident/CR/problem hits. */
interface EntityCardHit {
  icon: JSX.Element;
  idLabel?: string | null;
  subject: string;
  state?: string | null;
  assigneeName?: string;
}

interface Result {
  key: string;
  icon: JSX.Element;
  label: string;
  sublabel?: string;
  href: string;
  section: Section;
  /** Present only for "Cases" results — renders as a rich card instead of a plain row. */
  caseHit?: QuickCaseHit;
  /** Present only for Incident/Change-request/Problem results — see {@link EntityCardHit}. */
  entityHit?: EntityCardHit;
}

const RECENT_LIMIT = 8;

const isMac =
  typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform);

export default function QuickNav(): JSX.Element | null {
  const { isSignedIn } = useAsgardeo();
  const navigate = useNavTransition();
  const recents = useRecentViews();
  // Shrink the closed trigger (not the open palette) once something is
  // pinned, so PinnedTabs — which shares the header's flexible middle slot —
  // has room to actually show the pinned chips instead of getting squeezed.
  const hasPinned = recents.some((e) => e.pinned);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);

  // Shareable-link support: `?q=` opens the palette pre-filled with a search
  // (e.g. a link like `/?q=CS0440883` someone pastes to a colleague), and
  // `?goto=` additionally auto-jumps into the single matching record once
  // its search settles, instead of leaving the user to click it themselves.
  const [searchParams, setSearchParams] = useSearchParams();
  const [gotoTarget, setGotoTarget] = useState<string | null>(null);
  const consumedInitialParams = useRef(false);

  // Debounce the text fed to the case-search API so each keystroke doesn't fire
  // a request; the in-memory pinned/recent/page matching still reacts instantly
  // to `query`.
  const debouncedQuery = useDebouncedValue(query, 180);
  const trimmedQuery = query.trim();
  // Case hits lag the input by the debounce window, so `caseSearch.data` can
  // describe a previous query. Only surface (and allow navigating to) hits once
  // the query the API actually ran matches what's typed now — otherwise stale
  // results stay clickable during the debounce window or after the input shrinks.
  const caseHitsSettled = trimmedQuery === debouncedQuery.trim();

  // API-backed case lookup: a CS/WSO2 id (or any subject text) resolves to real
  // cases. Disabled until the query is long enough (see the hook).
  const caseSearch = useQuickCaseSearch(open ? debouncedQuery : "");
  // True while a case search is in flight (or its result is for a stale
  // query) — drives the "Cases" section's skeleton independently of whether
  // Pinned/Recent/Pages already have matches to show.
  const casesLoading =
    trimmedQuery.length >= QUICK_CASE_MIN_QUERY_LEN &&
    (caseSearch.isFetching || !caseHitsSettled);
  // Skeleton only while there's nothing to show yet — `isFetching` also
  // covers a background refetch of an already-settled, already-rendered
  // query (e.g. re-typing a query after the 15s staleTime), where
  // `caseSearch.data` still holds the previous results. Without this,
  // the skeleton block and the real "Cases" section would render together.
  const showCasesSkeleton = casesLoading && !caseSearch.data;

  // Same debounced query string fans out to the other searchable entity
  // kinds — one shared debounce (above) rather than each hook debouncing its
  // own copy, so a keystroke costs at most one query-string change, not four.
  // Incidents/CRs/problems are ServiceNow-only and comparatively rare hits,
  // so — unlike Cases — these don't get a dedicated skeleton: their sections
  // simply appear once data lands, same as Pinned/Recent/Pages.
  const incidentSearch = useQuickIncidentSearch(open ? debouncedQuery : "");
  const changeRequestSearch = useQuickChangeRequestSearch(
    open ? debouncedQuery : "",
  );
  const problemSearch = useQuickProblemSearch(open ? debouncedQuery : "");

  const inputRef = useRef<HTMLInputElement>(null);

  // Consume `?q=`/`?goto=` once per page load, not on every render — a
  // `setSearchParams` below removes them from the URL, and re-reading them
  // after that (e.g. from a stale closure) would just no-op harmlessly, but
  // this guard also stops a re-mount from re-opening the palette if the user
  // has already closed it once this session.
  useEffect(() => {
    if (!isSignedIn || consumedInitialParams.current) return;
    const q = searchParams.get("q");
    const goto = searchParams.get("goto");
    if (!q && !goto) return;
    consumedInitialParams.current = true;
    /* eslint-disable react-hooks/set-state-in-effect -- syncs palette state to an external one-shot source (the URL's initial q/goto params) */
    setOpen(true);
    setQuery(goto || q || "");
    if (goto) setGotoTarget(goto.trim());
    /* eslint-enable react-hooks/set-state-in-effect */
  }, [isSignedIn, searchParams]);

  // ⌘K / Ctrl+K toggles the palette — only while signed in, so we don't hijack
  // the browser shortcut on the sign-in screen (where the palette can't render).
  useEffect(() => {
    if (!isSignedIn) return;
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((o) => !o);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [isSignedIn]);

  // Focus the input once the palette has mounted. `autoFocus` alone can lose
  // a focus-trap race against the Modal claiming focus on open, leaving the
  // palette open but requiring a second click before typing works.
  useEffect(() => {
    if (!open) return;
    const id = requestAnimationFrame(() => inputRef.current?.focus());
    return () => cancelAnimationFrame(id);
  }, [open]);

  const results: Result[] = useMemo(() => {
    const q = trimmedQuery.toLowerCase();
    const match = (...parts: (string | undefined)[]) =>
      !q || parts.some((p) => p?.toLowerCase().includes(q));

    // Live case hits go first — when someone types a case id, the matching case
    // is the thing they want, ahead of pinned/recent/pages. Only shown once the
    // debounced query the API ran matches the current input, so stale hits never
    // stay clickable mid-typing.
    const cases: Result[] =
      caseHitsSettled && trimmedQuery.length >= QUICK_CASE_MIN_QUERY_LEN
        ? (caseSearch.data ?? []).map((c) => {
            const idLabel = caseIdLabel(c);
            return {
              key: `case-${c.id}`,
              icon: kindIcon("case", 16),
              label: idLabel || c.subject,
              sublabel: idLabel ? c.subject : undefined,
              href: `/cases/${c.id}`,
              section: "Cases" as const,
              caseHit: c,
            };
          })
        : [];

    // Same "only once the debounce settled" gating as Cases above, so a
    // stale incident/CR/problem hit never stays clickable mid-typing either.
    const incidents: Result[] =
      caseHitsSettled && trimmedQuery.length >= QUICK_INCIDENT_MIN_QUERY_LEN
        ? (incidentSearch.data ?? []).map((i) => ({
            key: `incident-${i.id}`,
            icon: kindIcon("incident", 16),
            label: i.number || i.subject,
            sublabel: i.number ? i.subject : undefined,
            href: `/operations/incidents/${i.id}`,
            section: "Incidents" as const,
            entityHit: {
              icon: kindIcon("incident", 16),
              idLabel: i.number,
              subject: i.subject,
              state: i.state,
              assigneeName: i.assigneeName,
            },
          }))
        : [];

    const changeRequests: Result[] =
      caseHitsSettled &&
      trimmedQuery.length >= QUICK_CHANGE_REQUEST_MIN_QUERY_LEN
        ? (changeRequestSearch.data ?? []).map((cr) => ({
            key: `cr-${cr.id}`,
            icon: kindIcon("change_request", 16),
            label: cr.number || cr.subject,
            sublabel: cr.number ? cr.subject : undefined,
            href: `/operations/change-requests/${cr.id}`,
            section: "Change Requests" as const,
            entityHit: {
              icon: kindIcon("change_request", 16),
              idLabel: cr.number,
              subject: cr.subject,
              state: cr.state,
              assigneeName: cr.assigneeName,
            },
          }))
        : [];

    const problems: Result[] =
      caseHitsSettled && trimmedQuery.length >= QUICK_PROBLEM_MIN_QUERY_LEN
        ? (problemSearch.data ?? []).map((p) => ({
            key: `problem-${p.id}`,
            icon: kindIcon("problem", 16),
            label: p.number || p.subject,
            sublabel: p.number ? p.subject : undefined,
            href: `/operations/problems/${p.id}`,
            section: "Problems" as const,
            entityHit: {
              icon: kindIcon("problem", 16),
              idLabel: p.number,
              subject: p.subject,
              state: p.state,
              assigneeName: p.assigneeName,
            },
          }))
        : [];

    // A pinned/recent entry for a case carries a severity/status snapshot
    // from when it was last visited — render it as the same rich card a live
    // case search hit gets, instead of a plain icon+label row.
    const toCaseHit = (e: RecentView): QuickCaseHit | undefined =>
      e.kind === "case" && e.caseHit ? { id: e.id, ...e.caseHit } : undefined;

    const pinned: Result[] = recents
      .filter((e) => e.pinned)
      .filter((e) => match(e.title, e.subtitle))
      .map((e) => ({
        key: `pin-${e.kind}-${e.id}`,
        icon: kindIcon(e.kind, 16),
        label: e.title,
        sublabel: e.subtitle,
        href: e.href,
        section: "Pinned",
        caseHit: toCaseHit(e),
      }));

    const recent: Result[] = recents
      .filter((e) => !e.pinned)
      .filter((e) => match(e.title, e.subtitle))
      .slice(0, RECENT_LIMIT)
      .map((e) => ({
        key: `rec-${e.kind}-${e.id}`,
        icon: kindIcon(e.kind, 16),
        label: e.title,
        sublabel: e.subtitle,
        href: e.href,
        section: "Recents",
        caseHit: toCaseHit(e),
      }));

    // Pages are worth surfacing when someone types a page name to jump
    // straight there, but listing every sidebar page on the empty-query
    // default view just duplicates the sidebar itself — so only show this
    // section once there's something to match against. Second-level tabs are
    // offered too (matching on either the tab or its section name), so
    // "incidents" jumps straight into the tab rather than to Operations.
    const pages: Result[] = q
      ? navigableNavNodes()
          .filter((i) => match(i.label, i.sublabel))
          .map((i) => ({
            key: `page-${i.id}`,
            icon: <i.icon size={16} />,
            label: i.label,
            sublabel: i.sublabel,
            href: i.href,
            section: "Pages" as const,
          }))
      : [];

    return [
      ...cases,
      ...incidents,
      ...changeRequests,
      ...problems,
      ...pinned,
      ...recent,
      ...pages,
    ];
  }, [
    recents,
    trimmedQuery,
    caseHitsSettled,
    caseSearch.data,
    incidentSearch.data,
    changeRequestSearch.data,
    problemSearch.data,
  ]);

  // Clamp at render so a stale index from shrinking results never points past
  // the end (avoids a setState-in-effect cascade).
  const safeActive = results.length ? Math.min(active, results.length - 1) : 0;

  const close = () => {
    setOpen(false);
    setQuery("");
    setActive(0);
  };

  const choose = (r: Result | undefined) => {
    if (!r) return;
    close();
    navigate(r.href);
  };

  // `?goto=` resolution: once every search this query feeds has settled (not
  // just the case search — an incident/CR/problem number should auto-jump
  // too), look for an exact (case-insensitive) match on whichever identifier
  // a person would actually paste into a link — a display number
  // (`CS0440883`/`INC0012345`/...), the internal WSO2 case id, or a raw
  // record id — across all four result sets combined. Exactly one match
  // navigates straight there; zero or multiple matches leave the palette
  // open on its normal search results so the user can pick, since a forced
  // jump would be wrong (or arbitrary) either way.
  useEffect(() => {
    if (!gotoTarget) return;
    const allSettled =
      caseHitsSettled &&
      !caseSearch.isFetching &&
      !incidentSearch.isFetching &&
      !changeRequestSearch.isFetching &&
      !problemSearch.isFetching;
    if (!allSettled) return;

    const target = gotoTarget.toLowerCase();
    const matchesTarget = (...ids: (string | null | undefined)[]) =>
      ids.some((id) => id?.toLowerCase() === target);

    const caseHref = (caseSearch.data ?? []).find((c) =>
      matchesTarget(c.id, c.caseNumber, c.wso2CaseId),
    );
    const incidentHref = (incidentSearch.data ?? []).find((i) =>
      matchesTarget(i.id, i.number),
    );
    const crHref = (changeRequestSearch.data ?? []).find((cr) =>
      matchesTarget(cr.id, cr.number),
    );
    const problemHref = (problemSearch.data ?? []).find((p) =>
      matchesTarget(p.id, p.number),
    );

    const matches = [
      caseHref && `/cases/${caseHref.id}`,
      incidentHref && `/operations/incidents/${incidentHref.id}`,
      crHref && `/operations/change-requests/${crHref.id}`,
      problemHref && `/operations/problems/${problemHref.id}`,
    ].filter((href): href is string => !!href);

    /* eslint-disable react-hooks/set-state-in-effect -- syncs palette state to the external goto/search resolution outcome, a one-shot action once the search settles */
    setGotoTarget(null);
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.delete("q");
        next.delete("goto");
        return next;
      },
      { replace: true },
    );

    if (matches.length === 1) {
      close();
      navigate(matches[0]);
    }
    /* eslint-enable react-hooks/set-state-in-effect */
    // matches.length === 0 or > 1: leave the palette open on its normal
    // search results rather than guessing which one was meant.
  }, [
    gotoTarget,
    caseHitsSettled,
    caseSearch.isFetching,
    caseSearch.data,
    incidentSearch.isFetching,
    incidentSearch.data,
    changeRequestSearch.isFetching,
    changeRequestSearch.data,
    problemSearch.isFetching,
    problemSearch.data,
    navigate,
    setSearchParams,
  ]);

  const onListKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive(results.length ? (safeActive + 1) % results.length : 0);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive(
        results.length ? (safeActive - 1 + results.length) % results.length : 0,
      );
    } else if (e.key === "Enter") {
      e.preventDefault();
      choose(results[safeActive]);
    }
  };

  if (!isSignedIn) return null;

  const shortcut = isMac ? "⌘K" : "Ctrl K";

  return (
    <>
      <ButtonBase
        onClick={() => setOpen(true)}
        aria-label="Search or jump to (open quick nav)"
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 1,
          height: 36,
          width: hasPinned
            ? { xs: 40, sm: 260, md: 340, lg: 420 }
            : { xs: 40, sm: 340, md: 460, lg: 600 },
          px: { xs: 0, sm: 1.25 },
          justifyContent: { xs: "center", sm: "flex-start" },
          borderRadius: 1,
          border: 1,
          borderColor: "divider",
          color: "text.secondary",
          flexShrink: 0,
          "&:hover": { bgcolor: "action.hover" },
        }}
      >
        <Search size={16} />
        <Typography
          variant="body2"
          noWrap
          sx={{ flex: 1, textAlign: "left", display: { xs: "none", sm: "block" } }}
        >
          Search or jump to…
        </Typography>
        <Box
          component="span"
          sx={{
            display: { xs: "none", sm: "block" },
            fontSize: 11,
            px: 0.5,
            borderRadius: 0.5,
            border: 1,
            borderColor: "divider",
            color: "text.secondary",
          }}
        >
          {shortcut}
        </Box>
      </ButtonBase>

      {/*
        A `Dialog`'s paper is deliberately styled by the theme with a more
        opaque background + heavier blur, so a modal reads clearly over a
        dimmed page. Customer-portal's search dropdown isn't a Dialog at all
        — it's a plain `Paper` (oxygen-ui's `MuiPaper.styleOverrides.root`
        gives it the lighter, translucent "acrylic" background + a light
        blur + a divider border for free). Using `Modal` + `Paper` here
        — the same primitives, from the same "@wso2/oxygen-ui" import —
        gets the identical glassy look instead of fighting Dialog's styling.
      */}
      <Modal
        open={open}
        onClose={close}
        slotProps={{ backdrop: { sx: { backgroundColor: "transparent" } } }}
      >
        <Paper
          elevation={3}
          sx={{
            position: "fixed",
            top: "10vh",
            left: "50%",
            transform: "translateX(-50%)",
            width: { xs: "calc(100% - 32px)", sm: "calc(100% - 64px)" },
            maxWidth: 760,
            maxHeight: "65vh",
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
            outline: "none",
          }}
        >
          <Box
            onKeyDown={onListKeyDown}
            sx={{ display: "flex", flexDirection: "column", flex: 1, minHeight: 0 }}
          >
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
                px: 2,
                py: 1.5,
                borderBottom: 1,
                borderColor: "divider",
              }}
            >
              <Search size={18} />
              <InputBase
                autoFocus
                inputRef={inputRef}
                fullWidth
                placeholder="Search cases, incidents, change requests, problems, or jump to pinned, recent, pages…"
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  setActive(0);
                }}
                inputProps={{ "aria-label": "Quick nav search" }}
              />
            </Box>

            <Box sx={{ overflowY: "auto", flex: 1, minHeight: 0, p: 2 }}>
              {showCasesSkeleton && (
                <Box sx={{ mb: results.length ? 2 : 0 }}>
                  <Typography
                    variant="subtitle2"
                    color="text.secondary"
                    sx={{ display: "block", pb: 0.75, fontWeight: 600 }}
                  >
                    Cases
                  </Typography>
                  <QuickNavResultSkeleton count={3} />
                </Box>
              )}
              {results.length === 0 ? (
                casesLoading ? null : (
                  <Box
                    sx={{
                      display: "flex",
                      flexDirection: "column",
                      alignItems: "center",
                      py: 3,
                    }}
                  >
                    <SearchNoResultsIcon
                      style={{ width: 140, height: "auto", marginBottom: 12 }}
                    />
                    <Typography variant="body2" color="text.secondary">
                      {trimmedQuery.length === 0
                        ? "Nothing pinned or recent yet. Start typing to search."
                        : "No matches."}
                    </Typography>
                  </Box>
                )
              ) : (
                results.map((r, i) => {
                  const newSection = i === 0 || results[i - 1].section !== r.section;
                  return (
                    <Box key={r.key} sx={{ mt: newSection && i !== 0 ? 2 : 0 }}>
                      {newSection && (
                        <Typography
                          variant="subtitle2"
                          color="text.secondary"
                          sx={{
                            display: "block",
                            pb: 0.75,
                            fontWeight: 600,
                          }}
                        >
                          {r.section}
                        </Typography>
                      )}
                      {r.caseHit ? (
                        <Box sx={{ pb: 1 }}>
                          <QuickNavCaseCard
                            hit={r.caseHit}
                            active={i === safeActive}
                            onMouseEnter={() => setActive(i)}
                            onClick={() => choose(r)}
                          />
                        </Box>
                      ) : r.entityHit ? (
                        <Box sx={{ pb: 1 }}>
                          <QuickNavEntityCard
                            icon={r.entityHit.icon}
                            idLabel={r.entityHit.idLabel}
                            subject={r.entityHit.subject}
                            state={r.entityHit.state}
                            assigneeName={r.entityHit.assigneeName}
                            active={i === safeActive}
                            onMouseEnter={() => setActive(i)}
                            onClick={() => choose(r)}
                          />
                        </Box>
                      ) : (
                        <Box sx={{ pb: 1 }}>
                          <Form.CardButton
                            selected={i === safeActive}
                            onMouseEnter={() => setActive(i)}
                            onClick={() => choose(r)}
                            sx={{
                              display: "flex",
                              flexDirection: "row",
                              alignItems: "center",
                              gap: 1.5,
                              p: 1.25,
                              width: "100%",
                              minWidth: 0,
                            }}
                          >
                            {r.icon}
                            <Box sx={{ flex: 1, minWidth: 0 }}>
                              <Typography variant="body2" noWrap>
                                {r.label}
                              </Typography>
                              {r.sublabel && (
                                <Typography
                                  variant="caption"
                                  color="text.secondary"
                                  noWrap
                                >
                                  {r.sublabel}
                                </Typography>
                              )}
                            </Box>
                          </Form.CardButton>
                        </Box>
                      )}
                    </Box>
                  );
                })
              )}
            </Box>
          </Box>
        </Paper>
      </Modal>
    </>
  );
}
