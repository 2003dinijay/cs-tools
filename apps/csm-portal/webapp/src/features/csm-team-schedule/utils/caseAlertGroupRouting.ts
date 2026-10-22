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

import type {
  CaseAlertChatRoom,
  CaseAlertGroupRoute,
  CaseAlertRouteOptions,
  CaseAlertSettings,
} from "../types";

/** Where an assignment group's incidents go, and why. */
export interface GroupDestination {
  /** "team:<key>", "rota:<code>", or "" for the default rota or the shared room. */
  target: string;
  /** The destination, for people. */
  name: string;
  /** Why: a redirect saved here, the file, the team's name, or nothing. */
  why: string;
  /** The alert service's default SRE rota (a default group). */
  defaultRota?: boolean;
  /** Not mapped: the shared room only. */
  unmapped?: boolean;
  route?: CaseAlertGroupRoute;
}

const lower = (x: string) => x.trim().toLowerCase();

/**
 * Where an SRE assignment group's incidents are paged, as the alert service
 * decides it: a redirect saved here, else its file (an alias or a default
 * group), else a team named after the group, else nobody (the shared room).
 */
export function sreGroupDestination(
  s: CaseAlertSettings,
  options: CaseAlertRouteOptions | undefined,
): (name: string, route?: CaseAlertGroupRoute) => GroupDestination {
  const routing = s.running?.groups?.SRE;
  const teams = (options?.teams ?? []).filter((t) => t.family === "SRE");
  const named = [...(options?.groups ?? []), ...(options?.teams ?? [])];
  const teamName = (key: string) => teams.find((t) => t.key === key)?.name ?? key;
  const rotaName = (code: string) => {
    const r = (options?.rotas ?? []).find((x) => x.code === code);
    return r && r.label !== code ? r.label : code;
  };
  return (name, route) => {
    if (route && route.enabled) {
      // The alert service matches a team by its key, so a redirect from a
      // name that already is the key changes nothing.
      const needless = !!route.teamKey && lower(route.groupName) === route.teamKey.toLowerCase();
      return route.teamKey
        ? {
            target: `team:${route.teamKey}`,
            name: teamName(route.teamKey),
            why: needless ? "Same team, not needed: it is alerted as itself anyway" : "Redirect saved here",
            route,
          }
        : {
            // The alert service takes a rota redirect only to its default
            // rota: the group becomes a default group.
            target: `rota:${route.rotaCode ?? ""}`,
            name: `Every team on the ${rotaName(route.rotaCode ?? "")} rota`,
            why: "Redirect saved here",
            defaultRota: true,
            route,
          };
    }
    const alias = routing?.aliases?.[lower(name)];
    if (alias) return { target: `team:${alias}`, name: teamName(alias), why: "From the alert service's file", route };
    if ((routing?.defaultGroups ?? []).some((g) => lower(g) === lower(name))) {
      return { target: "", name: "Default SRE rota", why: "From the alert service's file", defaultRota: true, route };
    }
    const team = named.find((t) => lower(t.key) === lower(name) || lower(t.name) === lower(name));
    if (team) return { target: `team:${team.key}`, name: team.name, why: "A team has this name", route };
    return { target: "", name: "Not mapped", why: "Not mapped: nobody is called", unmapped: true, route };
  };
}

/** The saved room of an SRE assignment group, when it has one. */
export function groupRoomOf(rooms: CaseAlertChatRoom[], groupId: string): CaseAlertChatRoom | undefined {
  return rooms.find((r) => r.ladder === "SRE" && r.groupId === groupId);
}

/** The other saved rooms whose webhook points at the same space as room's
 *  (the hint names the space), for "same space as". */
export function sameSpaceAs(rooms: CaseAlertChatRoom[], room: CaseAlertChatRoom): CaseAlertChatRoom[] {
  const space = spaceOf(room.webhookHint);
  if (!room.hasWebhook || !space) return [];
  return rooms.filter((r) => r.id !== room.id && r.hasWebhook && spaceOf(r.webhookHint) === space);
}

/** The space id of a webhook hint ("spaces/AAAA · …wxyz" -> "spaces/AAAA"). */
function spaceOf(hint: string | undefined): string {
  return (hint ?? "").split("·")[0].trim();
}

/**
 * What happens to a group's cards while it has no chat space of its own, as a
 * phrase ("uses the Apollo SRE space", "gets no card: ..."): the room of
 * the team it is routed to, the rooms that take incidents with no SRE team
 * (a default group), else the shared SRE room -- saved here, or from the alert
 * service's file when it reports one.
 */
export function sreGroupFallback(
  s: CaseAlertSettings,
  rooms: CaseAlertChatRoom[],
  dest: GroupDestination,
): string {
  const sre = rooms.filter((r) => r.ladder === "SRE" && !r.groupId && r.enabled);
  const running = s.running?.channels?.SRE?.teams ?? [];
  const teamSpace = (key: string): string | undefined => {
    const room = sre.find((r) => r.teamKey === key && r.hasWebhook);
    if (room) return `the ${room.label} space`;
    if (running.some((t) => t.team === key && t.room)) return `the ${key} team's space from the alert service's file`;
    return undefined;
  };
  const uses = (space: string) => `uses ${space}`;
  const shared = sre.find((r) => !r.teamKey && r.hasWebhook);
  const sharedWords = shared
    ? uses(`the shared SRE space (${shared.label})`)
    : s.running?.groups?.SRE?.sharedRoom === false
      ? "gets no card: the SRE ladder has no shared space"
      : uses("the shared SRE space from the alert service's file");
  if (dest.target.startsWith("team:")) {
    const space = teamSpace(dest.target.slice(5));
    return space ? uses(space) : sharedWords;
  }
  if (dest.defaultRota) {
    const teamless = (s.running?.groups?.SRE?.teamlessRooms ?? sre.filter((r) => r.teamless).map((r) => r.teamKey ?? ""))
      .map((k) => teamSpace(k))
      .filter((x): x is string => !!x);
    if (teamless.length > 0) return uses(teamless.join(" and "));
  }
  return sharedWords;
}
