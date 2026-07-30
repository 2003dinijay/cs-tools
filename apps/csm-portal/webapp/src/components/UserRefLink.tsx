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

import type { JSX } from "react";
import { Link as RouterLink } from "react-router";

interface UserRefLinkProps {
  /** Display name shown as the link text (or plain text when there's no email). */
  name: string;
  /** The user's email, used to build the `/people/:email` link. Absent/empty
   * renders plain text — a user with no email on file can't be looked up. */
  email?: string;
  /** Optional className passthrough for layout tweaks. */
  className?: string;
}

/**
 * Renders a person's name as a link to their profile page (`/people/:email`)
 * when an email is available, with the same hover-underline treatment
 * {@link RelativeTime} uses for its permalink anchor. Falls back to plain text
 * when there's no email to look the user up by.
 */
export default function UserRefLink({
  name,
  email,
  className,
}: UserRefLinkProps): JSX.Element {
  if (!email || !email.trim()) {
    return <span className={className}>{name}</span>;
  }

  return (
    <RouterLink
      to={`/people/${encodeURIComponent(email)}`}
      className={className}
      style={{
        color: "inherit",
        textDecoration: "none",
        cursor: "pointer",
      }}
      onMouseEnter={(e) => {
        (e.currentTarget as HTMLAnchorElement).style.textDecoration =
          "underline";
      }}
      onMouseLeave={(e) => {
        (e.currentTarget as HTMLAnchorElement).style.textDecoration = "none";
      }}
    >
      {name}
    </RouterLink>
  );
}
