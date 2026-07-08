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

import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";

dayjs.extend(relativeTime);

export const formatDate = (date: Date): string => dayjs(date).format("MMM D, YYYY h:mm A");

// dayjs renders an Invalid Date as "a month ago" rather than erroring, which silently masks
// unparseable timestamps as a plausible-looking value instead of surfacing the bug.
export const fromNow = (date: Date): string => (Number.isNaN(date.getTime()) ? "—" : dayjs(date).fromNow());

// The backend is ServiceNow-sourced and its timestamps are UTC but carry no zone marker
// (e.g. "2026-06-08 10:15:00" or "2026-06-08T10:15:00"). `new Date()` treats a zone-less
// date-time string as local time, so parsing it directly silently shifts it by the
// browser's UTC offset (or fails to parse at all, depending on runtime). Normalize to an
// explicit UTC ISO string before handing it to `Date` so the parsed instant is correct
// regardless of the viewer's timezone.
function normalizeBackendTimestamp(raw: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2}):(\d{2})(\.\d+)?$/.exec(raw);
  if (!match) return raw;

  const [, yyyy, mm, dd, hh, mi, ss, fractional = ""] = match;
  return `${yyyy}-${mm}-${dd}T${hh}:${mi}:${ss}${fractional}Z`;
}

export const parseBackendTimestamp = (raw: string): Date => new Date(normalizeBackendTimestamp(raw));

export const parseOptionalBackendTimestamp = (raw: string | null | undefined): Date | null =>
  raw ? parseBackendTimestamp(raw) : null;
